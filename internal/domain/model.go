// Package domain trägt die Begriffe und Regeln der Schichtplanung.
//
// Hier steht, was ein Tag, eine Schicht und ein Feiertag sind und was ein
// Wunsch der Oberfläche auslöst. Das Paket liest keine Datenbank, schreibt
// keine Antwort und kennt kein HTTP - es rechnet nur. Deshalb lässt es sich
// ohne Gerüst prüfen, und deshalb hängt alles andere von ihm ab und nicht
// umgekehrt.
package domain

// ── Data structures ───────────────────────────────────────────────────────────

type Employee struct {
	Name  string            `json:"name"`
	Team  string            `json:"team"`
	Color string            `json:"color"`
	Icon  string            `json:"icon"`
	Prefs map[string]string `json:"prefs"`
}

// DaySlot holds the entries of one day. Diese Fassung plant nur Schichten:
// Abwesenheiten (Urlaub, Krank, Elternzeit, Sonderurlaub) kommen nicht mehr
// vor. Vorhandene Zeilen dieser Art bleiben in der Datenbank liegen, werden
// aber nirgends mehr gelesen oder geschrieben.
type DaySlot struct {
	Frueh           []string `json:"frueh"`
	Normal          []string `json:"normal"`
	Spaet           []string `json:"spaet"`
	Rufbereitschaft []string `json:"rufbereitschaft"`
}

type SollBesetzung struct {
	Frueh           int `json:"frueh"`
	Normal          int `json:"normal"`
	Spaet           int `json:"spaet"`
	Rufbereitschaft int `json:"rufbereitschaft"`
}

type CustomHoliday struct {
	Date    string `json:"date"`
	Name    string `json:"name"`
	Country string `json:"country"`
}

// Template: map[personName]map[weekday(string)]shiftType
type Template map[string]map[string]string

type AppData struct {
	Mitarbeiter    []Employee          `json:"mitarbeiter"`
	Schichten      map[string]DaySlot  `json:"schichten"`
	Notizen        map[string]string   `json:"notizen"`
	Soll           SollBesetzung       `json:"soll"`
	CustomHolidays []CustomHoliday     `json:"custom_holidays"`
	Templates      map[string]Template `json:"templates"`
	RufKW          map[string]any      `json:"ruf_kw"`
}

func DefaultData() AppData {
	return AppData{
		Mitarbeiter:    []Employee{},
		Schichten:      map[string]DaySlot{},
		Notizen:        map[string]string{},
		Soll:           SollBesetzung{Frueh: 1, Normal: 0, Spaet: 1, Rufbereitschaft: 1},
		CustomHolidays: []CustomHoliday{},
		Templates:      map[string]Template{},
		RufKW:          map[string]any{},
	}
}

func EmptySlot() DaySlot {
	return DaySlot{
		Frueh:           []string{},
		Normal:          []string{},
		Spaet:           []string{},
		Rufbereitschaft: []string{},
	}
}

// Normalize fills in anything an older or hand-edited data file may be
// missing, so handlers never have to deal with nil maps.
func Normalize(d *AppData) {
	if d.Schichten == nil {
		d.Schichten = map[string]DaySlot{}
	}
	if d.Notizen == nil {
		d.Notizen = map[string]string{}
	}
	if d.CustomHolidays == nil {
		d.CustomHolidays = []CustomHoliday{}
	}
	if d.Templates == nil {
		d.Templates = map[string]Template{}
	}
	if d.Mitarbeiter == nil {
		d.Mitarbeiter = []Employee{}
	}
	if d.Soll == (SollBesetzung{}) {
		d.Soll = SollBesetzung{Frueh: 1, Normal: 0, Spaet: 1, Rufbereitschaft: 1}
	}
	d.RufKW = UnwrapRufKW(d.RufKW)
	for i := range d.Mitarbeiter {
		if d.Mitarbeiter[i].Color == "" {
			d.Mitarbeiter[i].Color = "#4a9eff"
		}
		if d.Mitarbeiter[i].Prefs == nil {
			d.Mitarbeiter[i].Prefs = map[string]string{}
		}
	}
}

// UnwrapRufKW repairs KW plans that were stored one level too deep as
// {"ruf_kw": {...}} by an earlier version, and never returns nil.
func UnwrapRufKW(m map[string]any) map[string]any {
	for len(m) == 1 {
		inner, ok := m["ruf_kw"].(map[string]any)
		if !ok {
			break
		}
		m = inner
	}
	if m == nil {
		return map[string]any{}
	}
	return m
}

// ── Slot helpers ──────────────────────────────────────────────────────────────

// ShiftChange ist ein einzelner Eintrag, der angelegt oder entfernt wird.
type ShiftChange struct {
	Date  string
	Shift string
	Name  string
}

// ChangeEntry ist eine Zeile des Änderungsverlaufs.
type ChangeEntry struct {
	Time   string `json:"time"`
	Action string `json:"action"`
	Detail string `json:"detail"`
}
