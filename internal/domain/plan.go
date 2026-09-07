package domain

import (
	"strconv"
	"time"
)

// Dieser Teil trägt die Planungsregeln: welche Einträge ein Wunsch auslöst,
// worüber vorher nachgefragt werden muss und was ein Template oder ein
// KW-Plan im Kalender bedeuten. Alles hier rechnet nur - es liest keine
// Datenbank, schreibt keine Antwort und kennt kein HTTP. Genau deshalb lässt
// es sich einzeln prüfen.

// ShiftRequest ist der Wunsch, den die Oberfläche schickt.
type ShiftRequest struct {
	Dates   []string
	Shift   string
	Name    string
	Action  string // add, toggle, remove, clear_shift
	Force   bool   // Rückfrage wurde bestätigt
	Replace bool   // ... und zwar die Rückfrage "Schicht ersetzen?"
}

// Rückfragen, die ein Tag auslösen kann.
const (
	AskHoliday = "holiday_conflict" // Feiertag des eigenen Teams
	AskReplace = "needs_confirm"    // steht schon in einer anderen Arbeitsschicht
)

// DayOutcome ist das Ergebnis für einen einzelnen Tag: entweder der neue
// Stand des Tages oder eine Rückfrage.
type DayOutcome struct {
	Slot     DaySlot
	Ask      string   // "" heißt: erledigt
	Holiday  Holiday  // gesetzt bei AskHoliday
	Blocking []string // gesetzt bei AskReplace
}

// HolidayWarning meldet einen Eintrag, der trotz Feiertag angelegt wurde.
type HolidayWarning struct {
	Date    string `json:"date"`
	Name    string `json:"name"`
	Holiday string `json:"holiday"`
	Country string `json:"country"`
}

// PlanResult fasst zusammen, was der Wunsch bedeutet.
type PlanResult struct {
	Days     map[string]DayOutcome
	Adds     []ShiftChange
	Removes  []ShiftChange
	Warnings []HolidayWarning
}

// PlanShifts entscheidet, was ein Schichtwunsch auslöst.
//
// days sind die betroffenen Tage in ihrem aktuellen Stand, hols die Feiertage
// dieser Tage und team das Team der Person. Die Funktion ändert nichts: sie
// liefert die neuen Tagesstände und die Änderungen, die dafür gespeichert
// werden müssen.
func PlanShifts(req ShiftRequest, days map[string]DaySlot, hols map[string]Holiday, team string) PlanResult {
	res := PlanResult{Days: map[string]DayOutcome{}}

	for _, date := range req.Dates {
		slot := days[date]
		drop := func(shift string) {
			if RemoveFromSlot(&slot, shift, req.Name) {
				res.Removes = append(res.Removes, ShiftChange{Date: date, Shift: shift, Name: req.Name})
			}
		}

		switch req.Action {
		case "add", "toggle":
			feld := SlotField(&slot, req.Shift)
			if feld == nil { // unbekannte Schicht - der Tag bleibt, wie er ist
				break
			}
			// Ein Umschalten auf einem bestehenden Eintrag trägt ihn wieder aus.
			if req.Action == "toggle" && contains(*feld, req.Name) {
				drop(req.Shift)
				break
			}

			hol, istFeiertag := hols[date]
			amEigenenFeiertag := istFeiertag && !hol.Bridge && hol.AppliesTo(team)

			if !req.Force {
				// Feiertag des eigenen Teams: nachfragen, nicht still eintragen.
				if amEigenenFeiertag {
					res.Days[date] = DayOutcome{Ask: AskHoliday, Holiday: hol}
					continue
				}
				// Steht schon in einer anderen Arbeitsschicht: erst fragen.
				if blocking := blockingShifts(&slot, req.Shift, req.Name); len(blocking) > 0 {
					res.Days[date] = DayOutcome{Ask: AskReplace, Blocking: blocking}
					continue
				}
			} else {
				if amEigenenFeiertag {
					res.Warnings = append(res.Warnings, HolidayWarning{
						Date: date, Name: req.Name, Holiday: hol.Name, Country: hol.Country,
					})
				}
				// Bestätigtes Ersetzen: die andere Arbeitsschicht wird abgegeben.
				// Die Rufbereitschaft bleibt - sie läuft daneben mit.
				if req.Replace {
					for _, shift := range blockingShifts(&slot, req.Shift, req.Name) {
						drop(shift)
					}
				}
			}
			if AddToSlot(&slot, req.Shift, req.Name) {
				res.Adds = append(res.Adds, ShiftChange{Date: date, Shift: req.Shift, Name: req.Name})
			}

		case "remove":
			drop(req.Shift)

		case "clear_shift":
			if feld := SlotField(&slot, req.Shift); feld != nil {
				for _, name := range *feld {
					res.Removes = append(res.Removes, ShiftChange{Date: date, Shift: req.Shift, Name: name})
				}
				*feld = []string{}
			}
		}

		res.Days[date] = DayOutcome{Slot: slot}
	}
	return res
}

// RufKWChanges übersetzt den Wochenplan in Kalendereinträge.
//
// Ein Jahr meint seine Kalenderwochen, nicht seine Kalendertage: die KW 01
// beginnt oft schon im Dezember davor. Diese Tage müssen mit, sonst bleiben
// sie für immer leer - beim Übertragen des Vorjahres zählen sie nicht mit,
// weil ihr Wochenschlüssel schon zum neuen Jahr gehört. Ein einzelner Monat
// (month 1-12) meint dagegen den Kalendermonat.
func RufKWChanges(plan map[string]any, year, month int) []ShiftChange {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -7)
	ende := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 7)
	nurISOJahr := true
	if month > 0 {
		start = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		ende = start.AddDate(0, 1, 0)
		nurISOJahr = false
	}

	var changes []ShiftChange
	for tag := start; tag.Before(ende); tag = tag.AddDate(0, 0, 1) {
		if isoJahr, _ := tag.ISOWeek(); nurISOJahr && isoJahr != year {
			continue
		}
		for _, name := range KWNames(plan[ISOWeekKey(tag)]) {
			changes = append(changes, ShiftChange{
				Date: tag.Format("2006-01-02"), Shift: "rufbereitschaft", Name: name,
			})
		}
	}
	return changes
}

// TemplatePlan ist das Ergebnis eines Autoplan-Laufs.
type TemplatePlan struct {
	Changes         []ShiftChange
	SkippedHoliday  int
	SkippedConflict int
}

// ApplyTemplate rechnet aus, was ein Template in einem Monat bedeutet.
//
// days ist der aktuelle Stand der betroffenen Tage und wächst beim Rechnen
// mit: wer schon durch dieselbe Runde eingeplant wurde, gilt als eingeplant.
// Übersprungen wird, wer am Feiertag seines Teams stünde oder an dem Tag
// bereits in einer anderen Arbeitsschicht steht - der Autoplan darf nicht
// still tun, was der Handbetrieb nur nach Rückfrage tut.
func ApplyTemplate(tmpl Template, year, month int, days map[string]DaySlot,
	hols map[string]Holiday, teams map[string]string) TemplatePlan {

	var plan TemplatePlan
	erster := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	letzter := erster.AddDate(0, 1, -1)

	for name, wochentage := range tmpl {
		for tag := erster; !tag.After(letzter); tag = tag.AddDate(0, 0, 1) {
			// Im Template ist Montag 0 und Sonntag 6.
			shift, gesetzt := wochentage[strconv.Itoa((int(tag.Weekday())+6)%7)]
			if !gesetzt || shift == "" || shift == "frei" {
				continue
			}
			date := tag.Format("2006-01-02")
			if hol, ok := hols[date]; ok && !hol.Bridge && hol.AppliesTo(teams[name]) {
				plan.SkippedHoliday++
				continue
			}
			slot := days[date]
			if len(blockingShifts(&slot, shift, name)) > 0 {
				plan.SkippedConflict++
				continue
			}
			if !AddToSlot(&slot, shift, name) {
				continue
			}
			days[date] = slot
			plan.Changes = append(plan.Changes, ShiftChange{Date: date, Shift: shift, Name: name})
		}
	}
	return plan
}
