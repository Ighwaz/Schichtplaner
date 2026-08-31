package main

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

// easter calculates Easter Sunday for a given year (Gregorian)
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

// getGermanHolidays returns public holidays for Baden-Württemberg
func getGermanHolidays(year int) map[string]Holiday {
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

// getIndianHolidays returns major Indian public holidays (fixed-date ones + key variable)
func getIndianHolidays(year int) map[string]Holiday {
	hols := map[string]Holiday{
		fmt.Sprintf("%d-01-26", year): {Name: "Republic Day", Country: "IN"},
		fmt.Sprintf("%d-08-15", year): {Name: "Independence Day", Country: "IN"},
		fmt.Sprintf("%d-10-02", year): {Name: "Gandhi Jayanti", Country: "IN"},
		fmt.Sprintf("%d-12-25", year): {Name: "Christmas Day", Country: "IN"},
	}
	// Holi (variable - day after Holika Dahan, roughly March)
	// Diwali (variable - October/November)
	// These are approximations for common years
	holiDates := map[int]string{
		2024: "2024-03-25", 2025: "2025-03-14", 2026: "2026-03-03",
		2027: "2027-03-22", 2028: "2028-03-10",
	}
	diwaliDates := map[int]string{
		2024: "2024-11-01", 2025: "2025-10-20", 2026: "2026-11-08",
		2027: "2027-10-29", 2028: "2028-10-17",
	}
	if d, ok := holiDates[year]; ok {
		hols[d] = Holiday{Name: "Holi", Country: "IN"}
	}
	if d, ok := diwaliDates[year]; ok {
		hols[d] = Holiday{Name: "Diwali", Country: "IN"}
	}
	return hols
}

// getBridgeDays finds bridge days (Brückentage) between holidays/weekends
func getBridgeDays(year int, existing map[string]Holiday) map[string]Holiday {
	bridges := map[string]Holiday{}
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := dateKey(d)
		dow := d.Weekday()
		// Only weekdays
		if dow == time.Saturday || dow == time.Sunday {
			continue
		}
		// Not already a holiday
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

// getAllHolidays combines DE + IN holidays for a year, plus custom holidays
func (a *App) getAllHolidays(year int, customs []CustomHoliday) map[string]Holiday {
	result := map[string]Holiday{}

	de := getGermanHolidays(year)
	in := getIndianHolidays(year)

	// Merge DE
	for k, v := range de {
		result[k] = v
	}

	// Merge IN - if same date, mark as both
	for k, v := range in {
		if existing, ok := result[k]; ok {
			// Same day - merge
			result[k] = Holiday{
				Name:    existing.Name + " / " + v.Name,
				Country: "DE+IN",
			}
		} else {
			result[k] = v
		}
	}

	// Add bridge days
	bridges := getBridgeDays(year, result)
	for k, v := range bridges {
		result[k] = v
	}

	// Custom holidays
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
