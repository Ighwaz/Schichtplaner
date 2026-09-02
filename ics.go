package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// icsLineLimit is the maximum line length RFC 5545 allows, in octets.
const icsLineLimit = 75

// writeICSLine writes one content line, folded the way RFC 5545 requires:
// continuation lines start with a space. Multi-byte characters are never
// split across a fold.
func writeICSLine(sb *strings.Builder, line string) {
	budget, n := icsLineLimit, 0
	for _, r := range line {
		size := utf8.RuneLen(r)
		if n+size > budget {
			sb.WriteString("\r\n ")
			budget = icsLineLimit - 1 // the leading space counts towards the limit
			n = 0
		}
		sb.WriteRune(r)
		n += size
	}
	sb.WriteString("\r\n")
}

// unfoldICS reads an ICS stream and joins the continuation lines back together
// before anything is parsed. Calendars from Outlook or Google fold every line
// past 75 octets, and an unfolded parser silently drops those entries.
func unfoldICS(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	var lines []string
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') && len(lines) > 0 {
			lines[len(lines)-1] += line[1:]
			continue
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

var shiftLabels = map[string]string{
	"frueh":           "Frühschicht",
	"normal":          "Normaldienst",
	"spaet":           "Spätschicht",
	"rufbereitschaft": "Rufbereitschaft",
}

var shiftTimes = map[string][2]string{
	"frueh": {"060000", "140000"},
	// Normaldienst ist Gleitzeit; die Zeiten sind nur ein Anhalt fuer den
	// Kalendereintrag, nicht die tatsaechliche Anwesenheit.
	"normal":          {"080000", "170000"},
	"spaet":           {"140000", "220000"},
	"rufbereitschaft": {"000000", "235959"},
}

func (a *App) handleExportICS(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	person := q.Get("person")
	yearStr := q.Get("year")
	monthStr := q.Get("month")

	d, ok := a.data(w)
	if !ok {
		return
	}

	sb := &strings.Builder{}
	sb.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Schichtplaner DE/IN//DE\r\nCALSCALE:GREGORIAN\r\n")

	now := time.Now().UTC().Format("20060102T150405Z")

	for date, slot := range d.Schichten {
		// Filter by year/month
		if yearStr != "" && !strings.HasPrefix(date, yearStr) {
			continue
		}
		if monthStr != "" {
			t, err := time.Parse("2006-01-02", date)
			if err != nil {
				continue
			}
			if fmt.Sprintf("%d", int(t.Month())) != monthStr {
				continue
			}
		}

		forEachShift(&slot, func(shift string, names *[]string) {
			for _, name := range *names {
				if person != "" && name != person {
					continue
				}
				label := shiftLabels[shift]
				times := shiftTimes[shift]
				dateClean := strings.ReplaceAll(date, "-", "")
				uid := fmt.Sprintf("%s-%s-%s@schichtplaner", dateClean, shift, strings.ReplaceAll(name, " ", "_"))

				sb.WriteString("BEGIN:VEVENT\r\n")
				writeICSLine(sb, "UID:"+uid)
				writeICSLine(sb, "DTSTAMP:"+now)
				writeICSLine(sb, fmt.Sprintf("DTSTART:%sT%s", dateClean, times[0]))
				writeICSLine(sb, fmt.Sprintf("DTEND:%sT%s", dateClean, times[1]))
				writeICSLine(sb, fmt.Sprintf("SUMMARY:%s – %s", name, label))
				writeICSLine(sb, "DESCRIPTION:"+label)
				sb.WriteString("END:VEVENT\r\n")
			}
		})
	}

	sb.WriteString("END:VCALENDAR\r\n")

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="schichtplan.ics"`)
	w.Write([]byte(sb.String()))
}

func (a *App) handleImportICS(w http.ResponseWriter, r *http.Request) {
	file, ok := uploadedFile(w, r, 16<<20)
	if !ok {
		return
	}
	defer file.Close()

	var changes []ShiftChange
	seen := map[ShiftChange]bool{}
	skipped := 0

	// Build reverse label map
	shiftByLabel := map[string]string{}
	for k, v := range shiftLabels {
		shiftByLabel[strings.ToLower(v)] = k
	}

	// Parse ICS - folded lines are joined first, see unfoldICS.
	lines, err := unfoldICS(file)
	if err != nil {
		writeJSON(w, map[string]string{"error": "Datei nicht lesbar: " + err.Error()})
		return
	}
	var (
		inEvent bool
		summary string
		dtstart string
	)

	for _, line := range lines {
		switch {
		case line == "BEGIN:VEVENT":
			inEvent = true
			summary = ""
			dtstart = ""
		case line == "END:VEVENT":
			inEvent = false
			if summary == "" || dtstart == "" {
				skipped++
				continue
			}
			// Parse date from DTSTART:20260401T060000 or DTSTART;TZID=...:20260401T060000
			raw := dtstart
			if i := strings.Index(raw, ":"); i >= 0 {
				raw = raw[i+1:]
			}
			if len(raw) < 8 {
				skipped++
				continue
			}
			dateStr := raw[:8]
			t, err := time.Parse("20060102", dateStr)
			if err != nil {
				skipped++
				continue
			}
			date := t.Format("2006-01-02")

			// Parse "Name – Schicht" from summary
			parts := strings.SplitN(summary, " – ", 2)
			if len(parts) != 2 {
				skipped++
				continue
			}
			name := strings.TrimSpace(parts[0])
			shiftLabel := strings.TrimSpace(parts[1])
			shift := shiftByLabel[strings.ToLower(shiftLabel)]
			if shift == "" {
				skipped++
				continue
			}

			change := ShiftChange{Date: date, Shift: shift, Name: name}
			if seen[change] {
				skipped++
				continue
			}
			seen[change] = true
			changes = append(changes, change)

		default:
			if !inEvent {
				continue
			}
			if strings.HasPrefix(line, "SUMMARY:") {
				summary = strings.TrimPrefix(line, "SUMMARY:")
			} else if strings.HasPrefix(line, "DTSTART") {
				dtstart = line
			}
		}
	}

	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	imported, err := s.AddShifts("import:ics", changes)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{
		"ok":       true,
		"imported": imported,
		"skipped":  skipped + len(changes) - imported,
	})
}
