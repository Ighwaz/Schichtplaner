package main

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var shiftLabels = map[string]string{
	"frueh":           "Frühschicht",
	"spaet":           "Spätschicht",
	"rufbereitschaft": "Rufbereitschaft",
	"urlaub":          "Urlaub",
	"krank":           "Krank",
	"elternzeit":      "Elternzeit",
	"sonderurlaub":    "Sonderurlaub",
}

var shiftTimes = map[string][2]string{
	"frueh":           {"060000", "140000"},
	"spaet":           {"140000", "220000"},
	"rufbereitschaft": {"000000", "235959"},
	"urlaub":          {"000000", "235959"},
	"krank":           {"000000", "235959"},
	"elternzeit":      {"000000", "235959"},
	"sonderurlaub":    {"000000", "235959"},
}

func (a *App) handleExportICS(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	person := q.Get("person")
	yearStr := q.Get("year")
	monthStr := q.Get("month")

	d := a.loadData()

	var sb strings.Builder
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

		for _, shift := range allShifts {
			f := slotField(&slot, shift)
			if f == nil {
				continue
			}
			for _, name := range *f {
				if person != "" && name != person {
					continue
				}
				label := shiftLabels[shift]
				times := shiftTimes[shift]
				dateClean := strings.ReplaceAll(date, "-", "")
				uid := fmt.Sprintf("%s-%s-%s@schichtplaner", dateClean, shift, strings.ReplaceAll(name, " ", "_"))

				sb.WriteString("BEGIN:VEVENT\r\n")
				sb.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
				sb.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
				sb.WriteString(fmt.Sprintf("DTSTART:%sT%s\r\n", dateClean, times[0]))
				sb.WriteString(fmt.Sprintf("DTEND:%sT%s\r\n", dateClean, times[1]))
				sb.WriteString(fmt.Sprintf("SUMMARY:%s – %s\r\n", name, label))
				sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", label))
				sb.WriteString("END:VEVENT\r\n")
			}
		}
	}

	sb.WriteString("END:VCALENDAR\r\n")

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="schichtplan.ics"`)
	w.Write([]byte(sb.String()))
}

func (a *App) handleImportICS(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(16 << 20)
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, map[string]string{"error": "Keine Datei"})
		return
	}
	defer file.Close()

	d := a.loadData()
	imported := 0
	skipped := 0

	// Build reverse label map
	shiftByLabel := map[string]string{}
	for k, v := range shiftLabels {
		shiftByLabel[strings.ToLower(v)] = k
	}

	// Parse ICS
	scanner := bufio.NewScanner(file)
	var (
		inEvent bool
		summary string
		dtstart string
	)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
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

			slot, ok := d.Schichten[date]
			if !ok {
				slot = emptySlot()
			}
			f := slotField(&slot, shift)
			if f != nil && !contains(*f, name) {
				*f = append(*f, name)
				d.Schichten[date] = slot
				imported++
			} else {
				skipped++
			}

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

	a.saveData(d)
	writeJSON(w, map[string]interface{}{
		"ok":       true,
		"imported": imported,
		"skipped":  skipped,
	})
}
