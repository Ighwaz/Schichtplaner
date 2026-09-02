package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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
	Mitarbeiter    []Employee             `json:"mitarbeiter"`
	Schichten      map[string]DaySlot     `json:"schichten"`
	Notizen        map[string]string      `json:"notizen"`
	Soll           SollBesetzung          `json:"soll"`
	CustomHolidays []CustomHoliday        `json:"custom_holidays"`
	Templates      map[string]Template    `json:"templates"`
	RufKW          map[string]interface{} `json:"ruf_kw"`
}

// ── Config ───────────────────────────────────────────────────────────────────

type Config struct {
	DataFolder string `json:"data_folder"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".schichtplaner_config.json")
}

func loadConfig() Config {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return Config{}
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg Config) {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath(), data, 0644)
}

// ── Data shapes ───────────────────────────────────────────────────────────────

// dataFileName is the plan file of earlier versions. It is imported into the
// database once and then no longer written to; see store.go.
const dataFileName = "schichtplan_daten.json"

func defaultData() AppData {
	return AppData{
		Mitarbeiter:    []Employee{},
		Schichten:      map[string]DaySlot{},
		Notizen:        map[string]string{},
		Soll:           SollBesetzung{Frueh: 1, Normal: 0, Spaet: 1, Rufbereitschaft: 1},
		CustomHolidays: []CustomHoliday{},
		Templates:      map[string]Template{},
		RufKW:          map[string]interface{}{},
	}
}

func emptySlot() DaySlot {
	return DaySlot{
		Frueh:           []string{},
		Normal:          []string{},
		Spaet:           []string{},
		Rufbereitschaft: []string{},
	}
}

// normalizeData fills in anything an older or hand-edited data file may be
// missing, so handlers never have to deal with nil maps.
func normalizeData(d *AppData) {
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
	d.RufKW = unwrapRufKW(d.RufKW)
	for i := range d.Mitarbeiter {
		if d.Mitarbeiter[i].Color == "" {
			d.Mitarbeiter[i].Color = "#4a9eff"
		}
		if d.Mitarbeiter[i].Prefs == nil {
			d.Mitarbeiter[i].Prefs = map[string]string{}
		}
	}
}

// unwrapRufKW repairs KW plans that were stored one level too deep as
// {"ruf_kw": {...}} by an earlier version, and never returns nil.
func unwrapRufKW(m map[string]interface{}) map[string]interface{} {
	for len(m) == 1 {
		inner, ok := m["ruf_kw"].(map[string]interface{})
		if !ok {
			break
		}
		m = inner
	}
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}

// ── Slot helpers ──────────────────────────────────────────────────────────────

func slotField(s *DaySlot, shift string) *[]string {
	switch shift {
	case "frueh":
		return &s.Frueh
	case "normal":
		return &s.Normal
	case "spaet":
		return &s.Spaet
	case "rufbereitschaft":
		return &s.Rufbereitschaft
	}
	return nil
}

// slotFor returns the day's slot, or an empty one if the day has no entries yet.
func slotFor(d *AppData, date string) DaySlot {
	if slot, ok := d.Schichten[date]; ok {
		return slot
	}
	return emptySlot()
}

// forEachShift calls fn for every shift list of a slot.
func forEachShift(slot *DaySlot, fn func(shift string, names *[]string)) {
	for _, shift := range allShifts {
		if f := slotField(slot, shift); f != nil {
			fn(shift, f)
		}
	}
}

// addToSlot adds name to one shift of a slot and reports whether that changed
// anything - an unknown shift or a name that is already there changes nothing.
func addToSlot(slot *DaySlot, shift, name string) bool {
	f := slotField(slot, shift)
	if f == nil || contains(*f, name) {
		return false
	}
	*f = append(*f, name)
	return true
}

// removeFromSlot removes name from one shift and reports whether it was there.
func removeFromSlot(slot *DaySlot, shift, name string) bool {
	f := slotField(slot, shift)
	if f == nil || !contains(*f, name) {
		return false
	}
	*f = remove(*f, name)
	return true
}

// blockingShifts lists the work shifts name already holds that day and that
// cannot be combined with shift. Rufbereitschaft runs alongside everything.
func blockingShifts(slot *DaySlot, shift, name string) []string {
	// Rufbereitschaft laeuft neben jeder Arbeitsschicht her und wird deshalb
	// weder blockiert noch blockiert sie selbst.
	if !workShifts[shift] || shift == "rufbereitschaft" {
		return nil
	}
	var blocking []string
	for _, s := range allShifts {
		if !workShifts[s] || s == shift || s == "rufbereitschaft" {
			continue
		}
		if f := slotField(slot, s); f != nil && contains(*f, name) {
			blocking = append(blocking, s)
		}
	}
	return blocking
}

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

func remove(arr []string, s string) []string {
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

// workShifts sind die Schichten, die miteinander kollidieren koennen.
// "normal" ist der Tagdienst mit Gleitzeit - er liegt zwischen Frueh und
// Spaet und schliesst beide aus.
var workShifts = map[string]bool{
	"frueh": true, "normal": true, "spaet": true, "rufbereitschaft": true,
}

var allShifts = []string{
	"frueh", "normal", "spaet", "rufbereitschaft",
}
