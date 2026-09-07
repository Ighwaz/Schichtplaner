package domain

import (
	"testing"
)

// Die Planungsregeln ohne HTTP und ohne Datenbank: Eingaben rein, Entscheidung
// raus. Genau dafür wurde der Kern herausgelöst.

func tag(frueh, spaet, ruf []string) DaySlot {
	return DaySlot{Frueh: frueh, Spaet: spaet, Rufbereitschaft: ruf}
}

func TestEintragenLegtGenauEinenEintragAn(t *testing.T) {
	res := PlanShifts(
		ShiftRequest{Dates: []string{"2026-09-01"}, Shift: "frueh", Name: "Bauer", Action: "add"},
		map[string]DaySlot{"2026-09-01": {}}, nil, "DE")

	if len(res.Adds) != 1 || res.Adds[0].Shift != "frueh" {
		t.Fatalf("Adds: %#v", res.Adds)
	}
	if len(res.Removes) != 0 {
		t.Fatalf("nichts sollte entfernt werden: %#v", res.Removes)
	}
	if got := res.Days["2026-09-01"].Slot.Frueh; len(got) != 1 || got[0] != "Bauer" {
		t.Fatalf("Tag danach: %#v", got)
	}
}

func TestUmschaltenAufEinemBestehendenEintragTraegtAus(t *testing.T) {
	res := PlanShifts(
		ShiftRequest{Dates: []string{"2026-09-01"}, Shift: "frueh", Name: "Bauer", Action: "toggle"},
		map[string]DaySlot{"2026-09-01": tag([]string{"Bauer"}, nil, nil)}, nil, "DE")

	if len(res.Removes) != 1 || len(res.Adds) != 0 {
		t.Fatalf("Adds %#v Removes %#v", res.Adds, res.Removes)
	}
}

func TestZweiteArbeitsschichtFragtNach(t *testing.T) {
	res := PlanShifts(
		ShiftRequest{Dates: []string{"2026-09-01"}, Shift: "frueh", Name: "Bauer", Action: "add"},
		map[string]DaySlot{"2026-09-01": tag(nil, []string{"Bauer"}, nil)}, nil, "DE")

	got := res.Days["2026-09-01"]
	if got.Ask != AskReplace {
		t.Fatalf("Rückfrage: %q", got.Ask)
	}
	if len(got.Blocking) != 1 || got.Blocking[0] != "spaet" {
		t.Fatalf("blockierend: %#v", got.Blocking)
	}
	if len(res.Adds) != 0 {
		t.Fatal("vor der Rückfrage darf nichts eingetragen werden")
	}
}

func TestBestaetigtesErsetzenGibtDieAlteSchichtAb(t *testing.T) {
	res := PlanShifts(
		ShiftRequest{
			Dates: []string{"2026-09-01"}, Shift: "frueh", Name: "Bauer",
			Action: "add", Force: true, Replace: true,
		},
		map[string]DaySlot{"2026-09-01": tag(nil, []string{"Bauer"}, []string{"Bauer"})}, nil, "DE")

	if len(res.Removes) != 1 || res.Removes[0].Shift != "spaet" {
		t.Fatalf("Removes: %#v", res.Removes)
	}
	if len(res.Adds) != 1 || res.Adds[0].Shift != "frueh" {
		t.Fatalf("Adds: %#v", res.Adds)
	}
	// Die Rufbereitschaft läuft daneben mit und bleibt.
	if got := res.Days["2026-09-01"].Slot.Rufbereitschaft; len(got) != 1 {
		t.Fatalf("Rufbereitschaft verloren: %#v", got)
	}
}

func TestFeiertagDesEigenenTeamsFragtNach(t *testing.T) {
	hols := map[string]Holiday{"2026-10-03": {Name: "Tag der Deutschen Einheit", Country: "DE"}}

	res := PlanShifts(
		ShiftRequest{Dates: []string{"2026-10-03"}, Shift: "frueh", Name: "Bauer", Action: "add"},
		map[string]DaySlot{"2026-10-03": {}}, hols, "DE")
	if got := res.Days["2026-10-03"].Ask; got != AskHoliday {
		t.Fatalf("Rückfrage: %q", got)
	}

	// Für das andere Team ist es ein Arbeitstag.
	res = PlanShifts(
		ShiftRequest{Dates: []string{"2026-10-03"}, Shift: "frueh", Name: "Nair", Action: "add"},
		map[string]DaySlot{"2026-10-03": {}}, hols, "IN")
	if got := res.Days["2026-10-03"].Ask; got != "" {
		t.Fatalf("unerwartete Rückfrage: %q", got)
	}
}

func TestBestaetigterFeiertagsEintragWirdGemeldet(t *testing.T) {
	hols := map[string]Holiday{"2026-10-03": {Name: "Tag der Deutschen Einheit", Country: "DE"}}
	res := PlanShifts(
		ShiftRequest{
			Dates: []string{"2026-10-03"}, Shift: "frueh", Name: "Bauer",
			Action: "add", Force: true,
		},
		map[string]DaySlot{"2026-10-03": {}}, hols, "DE")

	if len(res.Adds) != 1 {
		t.Fatalf("Adds: %#v", res.Adds)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Holiday != "Tag der Deutschen Einheit" {
		t.Fatalf("Warnung fehlt: %#v", res.Warnings)
	}
}

func TestBrueckentagFragtNicht(t *testing.T) {
	hols := map[string]Holiday{"2026-10-02": {Name: "Brückentag", Country: "DE", Bridge: true}}
	res := PlanShifts(
		ShiftRequest{Dates: []string{"2026-10-02"}, Shift: "frueh", Name: "Bauer", Action: "add"},
		map[string]DaySlot{"2026-10-02": {}}, hols, "DE")

	if got := res.Days["2026-10-02"].Ask; got != "" {
		t.Fatalf("Brückentag hat nachgefragt: %q", got)
	}
}

func TestRufbereitschaftLaeuftNebenDerArbeitsschicht(t *testing.T) {
	res := PlanShifts(
		ShiftRequest{Dates: []string{"2026-09-01"}, Shift: "rufbereitschaft", Name: "Bauer", Action: "add"},
		map[string]DaySlot{"2026-09-01": tag([]string{"Bauer"}, nil, nil)}, nil, "DE")

	if got := res.Days["2026-09-01"].Ask; got != "" {
		t.Fatalf("Rufbereitschaft hat nachgefragt: %q", got)
	}
	if len(res.Adds) != 1 {
		t.Fatalf("Adds: %#v", res.Adds)
	}
}

func TestSchichtLeerenNimmtAlleNamenHeraus(t *testing.T) {
	res := PlanShifts(
		ShiftRequest{Dates: []string{"2026-09-01"}, Shift: "frueh", Action: "clear_shift"},
		map[string]DaySlot{"2026-09-01": tag([]string{"Bauer", "Nair"}, nil, nil)}, nil, "DE")

	if len(res.Removes) != 2 {
		t.Fatalf("Removes: %#v", res.Removes)
	}
	if got := res.Days["2026-09-01"].Slot.Frueh; len(got) != 0 {
		t.Fatalf("Zeile nicht leer: %#v", got)
	}
}

func TestUnbekannteSchichtAendertNichts(t *testing.T) {
	res := PlanShifts(
		ShiftRequest{Dates: []string{"2026-09-01"}, Shift: "nachtdienst", Name: "Bauer", Action: "add"},
		map[string]DaySlot{"2026-09-01": {}}, nil, "DE")

	if len(res.Adds) != 0 || len(res.Removes) != 0 {
		t.Fatalf("Adds %#v Removes %#v", res.Adds, res.Removes)
	}
}

func TestTemplateUeberspringtFeiertagUndKonflikt(t *testing.T) {
	// Montag 5.10.2026 bis Sonntag 11.10.2026; im Template steht Montag=frueh.
	tmpl := Template{"Bauer": {"0": "frueh"}}
	hols := map[string]Holiday{"2026-10-05": {Name: "Testfeiertag", Country: "DE"}}
	days := map[string]DaySlot{}
	teams := map[string]string{"Bauer": "DE"}

	plan := ApplyTemplate(tmpl, 2026, 10, days, hols, teams)
	if plan.SkippedHoliday != 1 {
		t.Fatalf("Feiertag nicht übersprungen: %#v", plan)
	}
	// Vier Montage im Oktober 2026, einer davon Feiertag.
	if len(plan.Changes) != 3 {
		t.Fatalf("geplante Tage: %#v", plan.Changes)
	}

	// Wer an einem Montag schon Spätschicht hat, wird nicht still umgeplant.
	days2 := map[string]DaySlot{"2026-10-12": tag(nil, []string{"Bauer"}, nil)}
	plan2 := ApplyTemplate(tmpl, 2026, 10, days2, nil, teams)
	if plan2.SkippedConflict != 1 {
		t.Fatalf("Konflikt nicht übersprungen: %#v", plan2)
	}
}

func TestKWPlanReichtUeberDieJahresgrenze(t *testing.T) {
	// Die erste Kalenderwoche 2025 beginnt am Montag, dem 30.12.2024.
	plan := map[string]any{"2025-W01": []any{"Nair"}}

	changes := RufKWChanges(plan, 2025, 0)
	if len(changes) != 7 {
		t.Fatalf("Tage: %d (%#v)", len(changes), changes)
	}
	if changes[0].Date != "2024-12-30" {
		t.Fatalf("erster Tag: %s", changes[0].Date)
	}

	// Ein einzelner Monat meint den Kalendermonat.
	if got := RufKWChanges(plan, 2025, 1); len(got) != 5 {
		t.Fatalf("Januar: %d Tage (%#v)", len(got), got)
	}
}

func TestKWPlanNimmtEinzelnenNamenUndListe(t *testing.T) {
	einer := RufKWChanges(map[string]any{"2026-W10": "Nair"}, 2026, 0)
	liste := RufKWChanges(map[string]any{"2026-W10": []any{"Nair"}}, 2026, 0)
	if len(einer) != 7 || len(liste) != 7 {
		t.Fatalf("einzeln %d, Liste %d", len(einer), len(liste))
	}
	// Alles andere ist kein Name und wird übergangen.
	if got := RufKWChanges(map[string]any{"2026-W10": 42}, 2026, 0); len(got) != 0 {
		t.Fatalf("Unsinn wurde übernommen: %#v", got)
	}
}
