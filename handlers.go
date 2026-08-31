package main

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// fail reports a problem to the frontend, which shows it as a toast.
func fail(w http.ResponseWriter, err error) {
	writeJSON(w, map[string]string{"error": err.Error()})
}

// requireStore returns the open data store, or reports that no folder is set.
func (a *App) requireStore(w http.ResponseWriter) (*Store, bool) {
	if a.store == nil {
		writeJSON(w, map[string]string{"error": "Kein Datenordner gewählt"})
		return nil, false
	}
	return a.store, true
}

// data loads the whole plan, or reports why it could not be read.
func (a *App) data(w http.ResponseWriter) (AppData, bool) {
	s, ok := a.requireStore(w)
	if !ok {
		return AppData{}, false
	}
	d, err := s.Load()
	if err != nil {
		fail(w, err)
		return AppData{}, false
	}
	return d, true
}

// write runs one store operation and reports a failure instead of swallowing it.
func (a *App) write(w http.ResponseWriter, fn func(*Store) error) bool {
	s, ok := a.requireStore(w)
	if !ok {
		return false
	}
	if err := fn(s); err != nil {
		fail(w, err)
		return false
	}
	return true
}

// uploadedFile returns the "file" part of a multipart upload, or reports the
// error to the client and returns false.
func uploadedFile(w http.ResponseWriter, r *http.Request, maxMemory int64) (multipart.File, bool) {
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		writeJSON(w, map[string]string{"error": "Ungültiger Upload"})
		return nil, false
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, map[string]string{"error": "Keine Datei"})
		return nil, false
	}
	return file, true
}

func pathSegment(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimPrefix(s, "/")
	s, _ = url.PathUnescape(s)
	return s
}

// ── /api/data ─────────────────────────────────────────────────────────────────

func (a *App) handleGetData(w http.ResponseWriter, r *http.Request) {
	if d, ok := a.data(w); ok {
		writeJSON(w, d)
	}
}

// ── /api/mitarbeiter ─────────────────────────────────────────────────────────

// respondEmployees answers with the current employee list plus optional extras.
func (a *App) respondEmployees(w http.ResponseWriter, extra map[string]interface{}) {
	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	list, err := s.Employees()
	if err != nil {
		fail(w, err)
		return
	}
	out := map[string]interface{}{"ok": true, "mitarbeiter": list}
	for k, v := range extra {
		out[k] = v
	}
	writeJSON(w, out)
}

func (a *App) handleGetMitarbeiter(w http.ResponseWriter, r *http.Request) {
	if d, ok := a.data(w); ok {
		writeJSON(w, d.Mitarbeiter)
	}
}

func (a *App) handleAddMitarbeiter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Team  string `json:"team"`
		Color string `json:"color"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeJSON(w, map[string]string{"error": "Name erforderlich"})
		return
	}
	d, ok := a.data(w)
	if !ok {
		return
	}
	for _, m := range d.Mitarbeiter {
		if m.Name == name {
			writeJSON(w, map[string]string{"error": "Name bereits vorhanden"})
			return
		}
	}
	color := body.Color
	if color == "" {
		color = "#4a9eff"
	}
	if !a.write(w, func(s *Store) error {
		return s.AddEmployee(Employee{Name: name, Team: body.Team, Color: color, Prefs: map[string]string{}})
	}) {
		return
	}
	a.respondEmployees(w, nil)
}

func (a *App) handleUpdateMitarbeiter(w http.ResponseWriter, r *http.Request) {
	oldName := pathSegment(r.URL.Path, "/api/mitarbeiter")
	var body struct {
		Name  string `json:"name"`
		Team  string `json:"team"`
		Color string `json:"color"`
		Icon  string `json:"icon"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeJSON(w, map[string]string{"error": "Name erforderlich"})
		return
	}
	d, ok := a.data(w)
	if !ok {
		return
	}
	found := false
	for _, m := range d.Mitarbeiter {
		if m.Name == oldName {
			found = true
		} else if m.Name == body.Name {
			writeJSON(w, map[string]string{"error": "Name bereits vorhanden"})
			return
		}
	}
	if !found {
		writeJSON(w, map[string]string{"error": "Nicht gefunden"})
		return
	}
	if !a.write(w, func(s *Store) error {
		return s.UpdateEmployee(oldName, Employee{
			Name: body.Name, Team: body.Team, Color: body.Color, Icon: body.Icon,
		})
	}) {
		return
	}
	a.respondEmployees(w, nil)
}

func (a *App) handleDeleteMitarbeiter(w http.ResponseWriter, r *http.Request) {
	name := pathSegment(r.URL.Path, "/api/mitarbeiter")
	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	// The backup lets the frontend offer a restore when the name is re-added.
	backup, err := s.DeleteEmployee(name)
	if err != nil {
		fail(w, err)
		return
	}
	a.respondEmployees(w, map[string]interface{}{"backup": backup})
}

func (a *App) handleSetColor(w http.ResponseWriter, r *http.Request) {
	// /api/mitarbeiter/<name>/color
	name := pathSegment(strings.TrimSuffix(r.URL.Path, "/color"), "/api/mitarbeiter")
	var body struct {
		Color string `json:"color"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if a.write(w, func(s *Store) error { return s.SetColor(name, body.Color) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (a *App) handleSetPrefs(w http.ResponseWriter, r *http.Request) {
	name := pathSegment(strings.TrimSuffix(r.URL.Path, "/prefs"), "/api/mitarbeiter")
	var body struct {
		Prefs map[string]string `json:"prefs"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if a.write(w, func(s *Store) error { return s.SetPrefs(name, body.Prefs) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (a *App) handleRestoreMitarbeiter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string                     `json:"name"`
		Entries map[string]map[string]bool `json:"entries"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.Name == "" {
		writeJSON(w, map[string]string{"error": "Name erforderlich"})
		return
	}
	var changes []ShiftChange
	for date, shifts := range body.Entries {
		for shift, on := range shifts {
			if on {
				changes = append(changes, ShiftChange{Date: date, Shift: shift, Name: body.Name})
			}
		}
	}
	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	restored, err := s.AddShifts("mitarbeiter:wiederherstellen", changes)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "restored": restored})
}

// ── /api/schicht ─────────────────────────────────────────────────────────────

func (a *App) handleSchicht(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dates   []string `json:"dates"`
		Schicht string   `json:"schicht"`
		Name    string   `json:"name"`
		Action  string   `json:"action"`
		Force   bool     `json:"force"`
		// Replace is set when the user confirmed the "Schicht ersetzen?" dialog;
		// only then is an existing work shift of that day given up.
		Replace bool `json:"replace"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s, ok := a.requireStore(w)
	if !ok {
		return
	}

	hols, err := a.holidaysForDates(s, body.Dates)
	if err != nil {
		fail(w, err)
		return
	}
	empTeam, err := s.Team(body.Name)
	if err != nil {
		fail(w, err)
		return
	}

	results := map[string]interface{}{}
	holWarnings := []map[string]string{}
	var adds, removes []ShiftChange

	for _, date := range body.Dates {
		slot, err := s.Day(date)
		if err != nil {
			fail(w, err)
			return
		}
		drop := func(shift string) {
			if removeFromSlot(&slot, shift, body.Name) {
				removes = append(removes, ShiftChange{Date: date, Shift: shift, Name: body.Name})
			}
		}

		switch body.Action {
		case "add", "toggle":
			f := slotField(&slot, body.Schicht)
			if f == nil {
				break
			}
			// A toggle on an existing entry just removes it again.
			if body.Action == "toggle" && contains(*f, body.Name) {
				drop(body.Schicht)
				break
			}
			hol, isHoliday := hols[date]
			if !body.Force {
				// Holiday of the employee's own team - gesetzlich oder selbst
				// eingetragen: nachfragen, statt still einzutragen.
				if isHoliday && !hol.Bridge && hol.appliesTo(empTeam) {
					results[date] = map[string]interface{}{
						"error":   "holiday_conflict",
						"holiday": hol.Name,
						"country": hol.Country,
					}
					continue
				}
				// Already on a different work shift that day: ask before replacing.
				if blocking := blockingShifts(&slot, body.Schicht, body.Name); len(blocking) > 0 {
					results[date] = map[string]interface{}{
						"error":    "needs_confirm",
						"blocking": blocking,
					}
					continue
				}
			} else {
				if isHoliday && !hol.Bridge && hol.appliesTo(empTeam) {
					holWarnings = append(holWarnings, map[string]string{
						"date":    date,
						"name":    body.Name,
						"holiday": hol.Name,
						"country": hol.Country,
					})
				}
				// Confirmed replacement: give up the conflicting work shifts.
				if body.Replace {
					for _, shift := range blockingShifts(&slot, body.Schicht, body.Name) {
						drop(shift)
					}
				}
			}
			if addToSlot(&slot, body.Schicht, body.Name) {
				adds = append(adds, ShiftChange{Date: date, Shift: body.Schicht, Name: body.Name})
			}

		case "remove":
			drop(body.Schicht)

		case "clear_shift":
			if f := slotField(&slot, body.Schicht); f != nil {
				for _, name := range *f {
					removes = append(removes, ShiftChange{Date: date, Shift: body.Schicht, Name: name})
				}
				*f = []string{}
			}
		}

		results[date] = slot
	}

	if len(removes) > 0 {
		if _, err := s.RemoveShifts("schicht:"+body.Action, removes); err != nil {
			fail(w, err)
			return
		}
	}
	if len(adds) > 0 {
		if _, err := s.AddShifts("schicht:"+body.Action, adds); err != nil {
			fail(w, err)
			return
		}
	}

	writeJSON(w, map[string]interface{}{
		"results":      results,
		"hol_warnings": holWarnings,
	})
}

// holidaysForDates loads the holidays of every year the given dates touch.
func (a *App) holidaysForDates(s *Store, dates []string) (map[string]Holiday, error) {
	customs, err := s.CustomHolidays()
	if err != nil {
		return nil, err
	}
	years := map[int]bool{time.Now().Year(): true}
	for _, date := range dates {
		if len(date) < 4 {
			continue
		}
		if y, err := strconv.Atoi(date[:4]); err == nil {
			years[y] = true
		}
	}
	hols := map[string]Holiday{}
	for y := range years {
		for k, v := range a.getAllHolidays(y, customs) {
			hols[k] = v
		}
	}
	return hols, nil
}

// ── /api/soll ─────────────────────────────────────────────────────────────────

func (a *App) handleSoll(w http.ResponseWriter, r *http.Request) {
	var body SollBesetzung
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if a.write(w, func(s *Store) error { return s.SetSoll(body) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/notiz ────────────────────────────────────────────────────────────────

func (a *App) handleNotiz(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date string `json:"date"`
		Text string `json:"text"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if a.write(w, func(s *Store) error { return s.SetNote(body.Date, body.Text) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/paste ────────────────────────────────────────────────────────────────

func (a *App) handlePaste(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slot  DaySlot  `json:"slot"`
		Dates []string `json:"dates"`
		Mode  string   `json:"mode"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ok := a.write(w, func(s *Store) error {
		if body.Mode == "replace" {
			return s.ReplaceDays(body.Dates, body.Slot)
		}
		return s.MergeDays(body.Dates, body.Slot)
	})
	if ok {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/holidays/<year> ─────────────────────────────────────────────────────

func (a *App) handleHolidays(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(pathSegment(r.URL.Path, "/api/holidays"))
	if err != nil {
		http.Error(w, "invalid year", 400)
		return
	}
	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	customs, err := s.CustomHolidays()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, a.getAllHolidays(year, customs))
}

// ── /api/custom_holidays ─────────────────────────────────────────────────────

func (a *App) handleGetCustomHolidays(w http.ResponseWriter, r *http.Request) {
	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	list, err := s.CustomHolidays()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, list)
}

func (a *App) handleAddCustomHoliday(w http.ResponseWriter, r *http.Request) {
	var body CustomHoliday
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if a.write(w, func(s *Store) error { return s.AddCustomHoliday(body) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (a *App) handleDeleteCustomHoliday(w http.ResponseWriter, r *http.Request) {
	key := pathSegment(r.URL.Path, "/api/custom_holidays")
	parts := strings.SplitN(key, "|", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid key", 400)
		return
	}
	if a.write(w, func(s *Store) error { return s.DeleteCustomHoliday(parts[0], parts[1]) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/templates ───────────────────────────────────────────────────────────

func (a *App) handleGetTemplates(w http.ResponseWriter, r *http.Request) {
	if d, ok := a.data(w); ok {
		writeJSON(w, d.Templates)
	}
}

func (a *App) handleSaveTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string   `json:"name"`
		Template Template `json:"template"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.Name == "" {
		writeJSON(w, map[string]string{"error": "Name erforderlich"})
		return
	}
	if a.write(w, func(s *Store) error { return s.SaveTemplate(body.Name, body.Template) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (a *App) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	name := pathSegment(r.URL.Path, "/api/templates")
	if a.write(w, func(s *Store) error { return s.DeleteTemplate(name) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/autoplan ─────────────────────────────────────────────────────────────

func (a *App) handleAutoplan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year     int    `json:"year"`
		Month    int    `json:"month"`
		Template string `json:"template"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.Month < 1 || body.Month > 12 {
		writeJSON(w, map[string]string{"error": "Ungültiger Monat"})
		return
	}
	d, ok := a.data(w)
	if !ok {
		return
	}
	tmpl, exists := d.Templates[body.Template]
	if !exists {
		writeJSON(w, map[string]string{"error": "Template nicht gefunden"})
		return
	}

	hols := a.getAllHolidays(body.Year, d.CustomHolidays)
	empTeam := map[string]string{}
	for _, m := range d.Mitarbeiter {
		empTeam[m.Name] = m.Team
	}

	firstDay := time.Date(body.Year, time.Month(body.Month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	var changes []ShiftChange
	for name, days := range tmpl {
		for day := firstDay; !day.After(lastDay); day = day.AddDate(0, 0, 1) {
			// Template weekdays are Mon=0 .. Sun=6.
			shift, set := days[strconv.Itoa((int(day.Weekday())+6)%7)]
			if !set || shift == "" || shift == "frei" {
				continue
			}
			date := day.Format("2006-01-02")
			if hol, isHoliday := hols[date]; isHoliday && !hol.Bridge && hol.appliesTo(empTeam[name]) {
				continue
			}
			changes = append(changes, ShiftChange{Date: date, Shift: shift, Name: name})
		}
	}

	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	planned, err := s.AddShifts("autoplan:"+body.Template, changes)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "planned": planned})
}

// ── /api/ruf_kw ──────────────────────────────────────────────────────────────

func (a *App) handleGetRufKW(w http.ResponseWriter, r *http.Request) {
	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	plan, err := s.RufKW()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, plan)
}

func (a *App) handleSaveRufKW(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// The frontend posts {"ruf_kw": {"2026-W12": [...]}} - store the inner map,
	// otherwise the KW keys sit one level too deep and apply/reload lose them.
	plan := unwrapRufKW(body)
	if a.write(w, func(s *Store) error { return s.SaveRufKW(plan) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (a *App) handleApplyRufKW(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	plan, err := s.RufKW()
	if err != nil {
		fail(w, err)
		return
	}

	start := time.Date(body.Year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(body.Year+1, 1, 1, 0, 0, 0, 0, time.UTC)
	if body.Month > 0 {
		start = time.Date(body.Year, time.Month(body.Month), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)
	}

	var changes []ShiftChange
	for cur := start; cur.Before(end); cur = cur.AddDate(0, 0, 1) {
		for _, name := range kwNames(plan[isoWeekKey(cur)]) {
			changes = append(changes, ShiftChange{
				Date: cur.Format("2006-01-02"), Shift: "rufbereitschaft", Name: name,
			})
		}
	}

	applied, err := s.AddShifts("kw-plan:übertragen", changes)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "applied": applied})
}

// kwNames reads a KW entry, which is either a single name or a list of names.
func kwNames(raw interface{}) []string {
	switch v := raw.(type) {
	case string:
		return []string{v}
	case []interface{}:
		var names []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				names = append(names, s)
			}
		}
		return names
	}
	return nil
}

// isoWeekKey formats a date as the KW key used by the plan, e.g. 2026-W12.
func isoWeekKey(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// ── /api/snapshot ─────────────────────────────────────────────────────────────

func (a *App) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Schichten map[string]DaySlot `json:"schichten"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.Schichten == nil {
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	if a.write(w, func(s *Store) error { return s.ReplaceAllShifts(body.Schichten) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/history ──────────────────────────────────────────────────────────────

func (a *App) handleHistory(w http.ResponseWriter, r *http.Request) {
	s, ok := a.requireStore(w)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 200
	}
	entries, err := s.History(limit)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, entries)
}

// ── /api/holiday_coverage ─────────────────────────────────────────────────────

// handleHolidayCoverage reports for which years Holi and Diwali are tabulated,
// so the UI can point out that they have to be added by hand beyond that.
func (a *App) handleHolidayCoverage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]int{
		"in_movable_from": inMovableFirstYear,
		"in_movable_to":   inMovableLastYear,
	})
}

// ── /api/datadir ──────────────────────────────────────────────────────────────

func (a *App) handleGetDatadir(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"folder": a.dataFolder,
		"file":   a.dbPath(),
	})
}

func (a *App) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	folder, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Datenordner wählen",
	})
	if err != nil || folder == "" {
		writeJSON(w, map[string]string{"error": "Abgebrochen"})
		return
	}
	a.useFolder(w, folder)
}

func (a *App) handleSetDatadir(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Folder string `json:"folder"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.Folder == "" {
		writeJSON(w, map[string]string{"error": "Kein Ordner"})
		return
	}
	info, err := os.Stat(body.Folder)
	if err != nil || !info.IsDir() {
		writeJSON(w, map[string]string{"error": "Ordner nicht gefunden"})
		return
	}
	a.useFolder(w, body.Folder)
}

// useFolder switches to another data folder and remembers the choice.
func (a *App) useFolder(w http.ResponseWriter, folder string) {
	if err := a.setDataFolder(folder); err != nil {
		fail(w, err)
		return
	}
	cfg := loadConfig()
	cfg.DataFolder = folder
	saveConfig(cfg)
	writeJSON(w, map[string]interface{}{"ok": true, "folder": folder, "file": a.dbPath()})
}

// ── /api/export_data / import_data ───────────────────────────────────────────

func (a *App) handleExportData(w http.ResponseWriter, r *http.Request) {
	d, ok := a.data(w)
	if !ok {
		return
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="schichtplan_backup.json"`)
	w.Write(raw)
}

func (a *App) handleImportData(w http.ResponseWriter, r *http.Request) {
	file, ok := uploadedFile(w, r, 32<<20)
	if !ok {
		return
	}
	defer file.Close()

	var d AppData
	if err := json.NewDecoder(file).Decode(&d); err != nil {
		writeJSON(w, map[string]string{"error": "Ungültiges JSON: " + err.Error()})
		return
	}
	normalizeData(&d)

	if !a.write(w, func(s *Store) error { return s.ReplaceAll(d, "import:backup") }) {
		return
	}
	writeJSON(w, map[string]interface{}{
		"ok":          true,
		"mitarbeiter": len(d.Mitarbeiter),
		"tage":        len(d.Schichten),
		"schichten":   len(d.Schichten),
		"templates":   len(d.Templates),
	})
}

// ── /api/export_ics / import_ics are in ics.go ───────────────────────────────
