package domain

import (
	"testing"
	"time"
)

// Die beweglichen indischen Feiertage stehen in Tabellen. Der Test haelt fest,
// dass beide Tabellen denselben Zeitraum abdecken wie MovableIN* verspricht -
// sonst meldet die Oberflaeche eine Deckung, die es nicht gibt.
func TestHoliAndDiwaliTablesCoverTheSameYears(t *testing.T) {
	for year := MovableINFirstYear; year <= MovableINLastYear; year++ {
		if _, ok := holiDates[year]; !ok {
			t.Errorf("Holi %d fehlt", year)
		}
		if _, ok := diwaliDates[year]; !ok {
			t.Errorf("Diwali %d fehlt", year)
		}
	}
	if len(holiDates) != MovableINLastYear-MovableINFirstYear+1 {
		t.Errorf("Holi-Tabelle hat %d Einträge, erwartet %d",
			len(holiDates), MovableINLastYear-MovableINFirstYear+1)
	}
	// Every tabulated date has to fall in its own year and parse cleanly.
	for _, table := range []map[int]string{holiDates, diwaliDates} {
		for year, date := range table {
			d, err := time.Parse("2006-01-02", date)
			if err != nil {
				t.Errorf("%s ist kein gültiges Datum: %v", date, err)
			} else if d.Year() != year {
				t.Errorf("%s steht unter Jahr %d", date, year)
			}
		}
	}
}
