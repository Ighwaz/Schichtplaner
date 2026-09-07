package domain

import (
	"fmt"
	"time"
)

type Holiday struct {
	Name    string `json:"name"`
	Country string `json:"country"`
	Bridge  bool   `json:"bridge,omitempty"`
	Custom  bool   `json:"custom,omitempty"`
}

// AppliesTo sagt, ob der Feiertag jemanden aus diesem Team betrifft.
// Wer kein Team hat, wird von keinem Feiertag aufgehalten.
func (h Holiday) AppliesTo(team string) bool {
	return team != "" && (h.Country == team || h.Country == "DE+IN")
}

// easter rechnet den Ostersonntag eines Jahres aus (gregorianisch).
func easter(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h+l-7*m+114)%31 + 1)
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func dateKey(t time.Time) string {
	return t.Format("2006-01-02")
}

func addDays(t time.Time, n int) time.Time {
	return t.AddDate(0, 0, n)
}

// germanHolidays liefert die gesetzlichen Feiertage in Baden-Württemberg.
func germanHolidays(year int) map[string]Holiday {
	e := easter(year)
	hols := map[string]Holiday{
		fmt.Sprintf("%d-01-01", year): {Name: "Neujahr", Country: "DE"},
		fmt.Sprintf("%d-01-06", year): {Name: "Heilige Drei Könige", Country: "DE"},
		dateKey(addDays(e, -2)):       {Name: "Karfreitag", Country: "DE"},
		dateKey(e):                    {Name: "Ostersonntag", Country: "DE"},
		dateKey(addDays(e, 1)):        {Name: "Ostermontag", Country: "DE"},
		fmt.Sprintf("%d-05-01", year): {Name: "Tag der Arbeit", Country: "DE"},
		dateKey(addDays(e, 39)):       {Name: "Christi Himmelfahrt", Country: "DE"},
		dateKey(addDays(e, 49)):       {Name: "Pfingstsonntag", Country: "DE"},
		dateKey(addDays(e, 50)):       {Name: "Pfingstmontag", Country: "DE"},
		dateKey(addDays(e, 60)):       {Name: "Fronleichnam", Country: "DE"},
		fmt.Sprintf("%d-10-03", year): {Name: "Tag der Deutschen Einheit", Country: "DE"},
		fmt.Sprintf("%d-11-01", year): {Name: "Allerheiligen", Country: "DE"},
		fmt.Sprintf("%d-12-25", year): {Name: "1. Weihnachtstag", Country: "DE"},
		fmt.Sprintf("%d-12-26", year): {Name: "2. Weihnachtstag", Country: "DE"},
	}
	return hols
}

// Holi und Diwali folgen dem hinduistischen Lunisolarkalender und haben keine
// Formel, deshalb stehen sie als Tabelle. Quelle: qppstudio.net, dort steht
// jeweils der Tag, den Indien als gesetzlichen Feiertag begeht - bei Holi also
// Rangwali Holi (das Farbenfest), nicht der Abend Holika Dahan davor.
// Jenseits von MovableINLastYear fehlen beide schlicht; die Oberfläche weist
// darauf hin, damit man sie als eigene Feiertage nachträgt.
const (
	MovableINFirstYear = 2026
	MovableINLastYear  = 2036
)

var holiDates = map[int]string{
	2026: "2026-03-03", 2027: "2027-03-22", 2028: "2028-03-11",
	2029: "2029-03-01", 2030: "2030-03-20", 2031: "2031-03-09",
	2032: "2032-03-27", 2033: "2033-03-16", 2034: "2034-03-05",
	2035: "2035-03-24", 2036: "2036-03-12",
}

var diwaliDates = map[int]string{
	2026: "2026-11-08", 2027: "2027-10-29", 2028: "2028-10-17",
	2029: "2029-11-05", 2030: "2030-10-26", 2031: "2031-11-14",
	2032: "2032-11-02", 2033: "2033-10-22", 2034: "2034-11-10",
	2035: "2035-10-30", 2036: "2036-10-18",
}

// indianHolidays liefert die indischen Feiertage mit festem Datum und dazu
// Holi und Diwali für die Jahre, für die sie tabelliert sind.
func indianHolidays(year int) map[string]Holiday {
	hols := map[string]Holiday{
		fmt.Sprintf("%d-01-26", year): {Name: "Republic Day", Country: "IN"},
		fmt.Sprintf("%d-08-15", year): {Name: "Independence Day", Country: "IN"},
		fmt.Sprintf("%d-10-02", year): {Name: "Gandhi Jayanti", Country: "IN"},
		fmt.Sprintf("%d-12-25", year): {Name: "Christmas Day", Country: "IN"},
	}
	if d, ok := holiDates[year]; ok {
		hols[d] = Holiday{Name: "Holi", Country: "IN"}
	}
	if d, ok := diwaliDates[year]; ok {
		hols[d] = Holiday{Name: "Diwali", Country: "IN"}
	}
	return hols
}

// bridgeDays findet Brückentage zwischen Feiertagen und Wochenenden.
func bridgeDays(year int, existing map[string]Holiday) map[string]Holiday {
	bridges := map[string]Holiday{}
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := dateKey(d)
		dow := d.Weekday()
		// Nur Werktage
		if dow == time.Saturday || dow == time.Sunday {
			continue
		}
		// Und nur, was nicht selbst schon Feiertag ist
		if _, ok := existing[key]; ok {
			continue
		}

		prev := d.AddDate(0, 0, -1)
		next := d.AddDate(0, 0, 1)
		prevKey := dateKey(prev)
		nextKey := dateKey(next)
		prevDow := prev.Weekday()
		nextDow := next.Weekday()

		isPrevFree := prevDow == time.Saturday || prevDow == time.Sunday
		if _, ok := existing[prevKey]; ok {
			isPrevFree = true
		}
		isNextFree := nextDow == time.Saturday || nextDow == time.Sunday
		if _, ok := existing[nextKey]; ok {
			isNextFree = true
		}

		if isPrevFree && isNextFree {
			bridges[key] = Holiday{Name: "Brückentag", Country: "DE", Bridge: true}
		}
	}
	return bridges
}

// AllHolidays liefert alle Feiertage eines Jahres: gesetzliche in DE und IN,
// Brückentage und die selbst eingetragenen.
func AllHolidays(year int, customs []CustomHoliday) map[string]Holiday {
	result := map[string]Holiday{}

	de := germanHolidays(year)
	in := indianHolidays(year)

	// Deutschland
	for k, v := range de {
		result[k] = v
	}

	// Indien - fällt es auf denselben Tag, gilt der Tag für beide Teams
	for k, v := range in {
		if existing, ok := result[k]; ok {
			// Derselbe Tag: beide Namen zusammenziehen
			result[k] = Holiday{
				Name:    existing.Name + " / " + v.Name,
				Country: "DE+IN",
			}
		} else {
			result[k] = v
		}
	}

	// Brückentage
	bridges := bridgeDays(year, result)
	for k, v := range bridges {
		result[k] = v
	}

	// Selbst eingetragene Feiertage
	for _, ch := range customs {
		t, err := time.Parse("2006-01-02", ch.Date)
		if err != nil {
			continue
		}
		if t.Year() != year {
			continue
		}
		result[ch.Date] = Holiday{
			Name:    ch.Name,
			Country: ch.Country,
			Custom:  true,
		}
	}

	return result
}
