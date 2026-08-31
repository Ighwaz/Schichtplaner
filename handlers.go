package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
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

func pathSegment(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimPrefix(s, "/")
	s, _ = url.PathUnescape(s)
	return s
}

// ── /api/data ─────────────────────────────────────────────────────────────────

func (a *App) handleGetData(w http.ResponseWriter, r *http.Request) {
	d := a.loadData()
	writeJSON(w, d)
}

// ── /api/mitarbeiter ─────────────────────────────────────────────────────────

func (a *App) handleGetMitarbeiter(w http.ResponseWriter, r *http.Request) {
	d := a.loadData()
	writeJSON(w, d.Mitarbeiter)
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

	d := a.loadData()
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
	d.Mitarbeiter = append(d.Mitarbeiter, Employee{
		Name:  name,
		Team:  body.Team,
		Color: color,
		Icon:  "",
		Prefs: map[string]string{},
	})
	a.saveData(d)
	writeJSON(w, map[string]interface{}{"ok": true, "mitarbeiter": d.Mitarbeiter})
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
	d := a.loadData()
	if body.Name != oldName {
		for _, m := range d.Mitarbeiter {
			if m.Name == body.Name {
				writeJSON(w, map[string]string{"error": "Name bereits vorhanden"})
				return
			}
		}
	}
	found := false
	for i, m := range d.Mitarbeiter {
		if m.Name == oldName {
			d.Mitarbeiter[i].Name = body.Name
			d.Mitarbeiter[i].Team = body.Team
			if body.Color != "" {
				d.Mitarbeiter[i].Color = body.Color
			}
			d.Mitarbeiter[i].Icon = body.Icon
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, map[string]string{"error": "Nicht gefunden"})
		return
	}
	if body.Name != oldName {
		renameEmployee(&d, oldName, body.Name)
	}
	a.saveData(d)
	writeJSON(w, map[string]interface{}{"ok": true, "mitarbeiter": d.Mitarbeiter})
}

// renameEmployee rewrites every reference to oldName across shifts,
// templates and the Rufbereitschafts-KW plan.
func renameEmployee(d *AppData, oldName, newName string) {
	for date, slot := range d.Schichten {
		for _, shift := range allShifts {
			f := slotField(&slot, shift)
			if f == nil {
				continue
			}
			for i, n := range *f {
				if n == oldName {
					(*f)[i] = newName
				}
			}
		}
		d.Schichten[date] = slot
	}
	for tName, tmpl := range d.Templates {
		if days, ok := tmpl[oldName]; ok {
			delete(tmpl, oldName)
			tmpl[newName] = days
			d.Templates[tName] = tmpl
		}
	}
	for kw, raw := range d.RufKW {
		switch v := raw.(type) {
		case string:
			if v == oldName {
				d.RufKW[kw] = newName
			}
		case []interface{}:
			for i, item := range v {
				if s, ok := item.(string); ok && s == oldName {
					v[i] = newName
				}
			}
			d.RufKW[kw] = v
		}
	}
}
func (a *App) handleDeleteMitarbeiter(w http.ResponseWriter, r *http.Request) {
	name := pathSegment(r.URL.Path, "/api/mitarbeiter")
	d := a.loadData()
	newList := []Employee{}
	for _, m := range d.Mitarbeiter {
		if m.Name != name {
			newList = append(newList, m)
		}
	}
	d.Mitarbeiter = newList

	// Remove the name from every shift and remember where it was, so the
	// frontend can offer to restore the entries when the name is re-added.
	backup := map[string]map[string]bool{}
	for date, slot := range d.Schichten {
		changed := false
		for _, shift := range allShifts {
			f := slotField(&slot, shift)
			if f == nil || !contains(*f, name) {
				continue
			}
			if backup[date] == nil {
				backup[date] = map[string]bool{}
			}
			backup[date][shift] = true
			*f = remove(*f, name)
			changed = true
		}
		if changed {
			d.Schichten[date] = slot
		}
	}
	a.saveData(d)
	writeJSON(w, map[string]interface{}{"ok": true, "mitarbeiter": d.Mitarbeiter, "backup": backup})
}
func (a *App) handleSetColor(w http.ResponseWriter, r *http.Request) {
	// /api/mitarbeiter/<name>/color
	path := strings.TrimSuffix(r.URL.Path, "/color")
	name, _ := url.PathUnescape(strings.TrimPrefix(path, "/api/mitarbeiter/"))
	var body struct {
		Color string `json:"color"`
	}
	readJSON(r, &body)
	d := a.loadData()
	for i, m := range d.Mitarbeiter {
		if m.Name == name {
			d.Mitarbeiter[i].Color = body.Color
			break
		}
	}
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *App) handleSetPrefs(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/prefs")
	name, _ := url.PathUnescape(strings.TrimPrefix(path, "/api/mitarbeiter/"))
	var body struct {
		Prefs map[string]string `json:"prefs"`
	}
	readJSON(r, &body)
	d := a.loadData()
	for i, m := range d.Mitarbeiter {
		if m.Name == name {
			d.Mitarbeiter[i].Prefs = body.Prefs
			break
		}
	}
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
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
	d := a.loadData()
	restored := 0
	for date, shifts := range body.Entries {
		slot, ok := d.Schichten[date]
		if !ok {
			slot = emptySlot()
		}
		for shift, on := range shifts {
			if !on {
				continue
			}
			f := slotField(&slot, shift)
			if f != nil && !contains(*f, body.Name) {
				*f = append(*f, body.Name)
				restored++
			}
		}
		d.Schichten[date] = slot
	}
	a.saveData(d)
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
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	d := a.loadData()
	// Load holidays for every year the request touches; a range may span years,
	// and a malformed date must not slice out of bounds.
	years := map[int]bool{time.Now().Year(): true}
	for _, date := range body.Dates {
		if len(date) < 4 {
			continue
		}
		if y, err := strconv.Atoi(date[:4]); err == nil {
			years[y] = true
		}
	}
	hols := map[string]Holiday{}
	for y := range years {
		for k, v := range a.getAllHolidays(y, d.CustomHolidays) {
			hols[k] = v
		}
	}

	results := map[string]interface{}{}
	holWarnings := []map[string]string{}

	// Find employee's team
	var empTeam string
	for _, m := range d.Mitarbeiter {
		if m.Name == body.Name {
			empTeam = m.Team
			break
		}
	}

	for _, date := range body.Dates {
		slot, ok := d.Schichten[date]
		if !ok {
			slot = emptySlot()
		}

		switch body.Action {
		case "add":
			// Holiday conflict check
			if !body.Force {
				if hol, exists := hols[date]; exists && !hol.Bridge && !hol.Custom {
					if empTeam != "" && (hol.Country == empTeam || hol.Country == "DE+IN") {
						results[date] = map[string]interface{}{
							"error":   "holiday_conflict",
							"holiday": hol.Name,
							"country": hol.Country,
						}
						continue
					}
				}
			} else {
				// Track warning
				if hol, exists := hols[date]; exists && !hol.Bridge {
					if empTeam != "" && (hol.Country == empTeam || hol.Country == "DE+IN") {
						holWarnings = append(holWarnings, map[string]string{
							"date":    date,
							"name":    body.Name,
							"holiday": hol.Name,
							"country": hol.Country,
						})
					}
				}
			}

			// Conflict with existing work shifts (needs_confirm)
			if workShifts[body.Schicht] && !body.Force {
				var blocking []string
				for _, s := range allShifts {
					if workShifts[s] && s != body.Schicht && s != "rufbereitschaft" {
						f := slotField(&slot, s)
						if f != nil && contains(*f, body.Name) {
							blocking = append(blocking, s)
						}
					}
				}
				if len(blocking) > 0 {
					results[date] = map[string]interface{}{
						"error":    "needs_confirm",
						"blocking": blocking,
					}
					continue
				}
			}

			f := slotField(&slot, body.Schicht)
			if f != nil && !contains(*f, body.Name) {
				*f = append(*f, body.Name)
			}

		case "remove":
			f := slotField(&slot, body.Schicht)
			if f != nil {
				*f = remove(*f, body.Name)
			}

		case "toggle":
			f := slotField(&slot, body.Schicht)
			if f == nil {
				break
			}
			if contains(*f, body.Name) {
				*f = remove(*f, body.Name)
			} else {
				// Check conflict
				if workShifts[body.Schicht] && !body.Force {
					var blocking []string
					for _, s := range allShifts {
						if workShifts[s] && s != body.Schicht && s != "rufbereitschaft" {
							sf := slotField(&slot, s)
							if sf != nil && contains(*sf, body.Name) {
								blocking = append(blocking, s)
							}
						}
					}
					if len(blocking) > 0 {
						results[date] = map[string]interface{}{
							"error":    "needs_confirm",
							"blocking": blocking,
						}
						continue
					}
				}
				*f = append(*f, body.Name)
			}

		case "clear_shift":
			f := slotField(&slot, body.Schicht)
			if f != nil {
				*f = []string{}
			}
		}

		d.Schichten[date] = slot
		results[date] = slot
	}

	a.saveData(d)
	writeJSON(w, map[string]interface{}{
		"results":      results,
		"hol_warnings": holWarnings,
	})
}

// ── /api/soll ─────────────────────────────────────────────────────────────────

func (a *App) handleSoll(w http.ResponseWriter, r *http.Request) {
	var body SollBesetzung
	readJSON(r, &body)
	d := a.loadData()
	d.Soll = body
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}

// ── /api/notiz ────────────────────────────────────────────────────────────────

func (a *App) handleNotiz(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date string `json:"date"`
		Text string `json:"text"`
	}
	readJSON(r, &body)
	d := a.loadData()
	if body.Text == "" {
		delete(d.Notizen, body.Date)
	} else {
		d.Notizen[body.Date] = body.Text
	}
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}

// ── /api/paste ────────────────────────────────────────────────────────────────

func (a *App) handlePaste(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slot  DaySlot  `json:"slot"`
		Dates []string `json:"dates"`
		Mode  string   `json:"mode"`
	}
	readJSON(r, &body)
	d := a.loadData()
	for _, date := range body.Dates {
		if body.Mode == "replace" {
			d.Schichten[date] = body.Slot
		} else {
			slot, ok := d.Schichten[date]
			if !ok {
				slot = emptySlot()
			}
			// Merge
			for _, s := range allShifts {
				src := slotField(&body.Slot, s)
				dst := slotField(&slot, s)
				if src != nil && dst != nil {
					for _, name := range *src {
						if !contains(*dst, name) {
							*dst = append(*dst, name)
						}
					}
				}
			}
			d.Schichten[date] = slot
		}
	}
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}

// ── /api/holidays/<year> ─────────────────────────────────────────────────────

func (a *App) handleHolidays(w http.ResponseWriter, r *http.Request) {
	yearStr := strings.TrimPrefix(r.URL.Path, "/api/holidays/")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "invalid year", 400)
		return
	}
	d := a.loadData()
	hols := a.getAllHolidays(year, d.CustomHolidays)
	writeJSON(w, hols)
}

// ── /api/custom_holidays ─────────────────────────────────────────────────────

func (a *App) handleGetCustomHolidays(w http.ResponseWriter, r *http.Request) {
	d := a.loadData()
	writeJSON(w, d.CustomHolidays)
}

func (a *App) handleAddCustomHoliday(w http.ResponseWriter, r *http.Request) {
	var body CustomHoliday
	readJSON(r, &body)
	d := a.loadData()
	// Remove duplicates
	filtered := []CustomHoliday{}
	for _, ch := range d.CustomHolidays {
		if !(ch.Date == body.Date && ch.Name == body.Name) {
			filtered = append(filtered, ch)
		}
	}
	filtered = append(filtered, body)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Date < filtered[j].Date
	})
	d.CustomHolidays = filtered
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *App) handleDeleteCustomHoliday(w http.ResponseWriter, r *http.Request) {
	key, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/custom_holidays/"))
	parts := strings.SplitN(key, "|", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid key", 400)
		return
	}
	date, name := parts[0], parts[1]
	d := a.loadData()
	filtered := []CustomHoliday{}
	for _, ch := range d.CustomHolidays {
		if !(ch.Date == date && ch.Name == name) {
			filtered = append(filtered, ch)
		}
	}
	d.CustomHolidays = filtered
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}

// ── /api/templates ───────────────────────────────────────────────────────────

func (a *App) handleGetTemplates(w http.ResponseWriter, r *http.Request) {
	d := a.loadData()
	writeJSON(w, d.Templates)
}

func (a *App) handleSaveTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string   `json:"name"`
		Template Template `json:"template"`
	}
	readJSON(r, &body)
	d := a.loadData()
	d.Templates[body.Name] = body.Template
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *App) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	name, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/templates/"))
	d := a.loadData()
	delete(d.Templates, name)
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}

// ── /api/autoplan ─────────────────────────────────────────────────────────────

func (a *App) handleAutoplan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year      int    `json:"year"`
		Month     int    `json:"month"`
		Template  string `json:"template"`
		Overwrite bool   `json:"overwrite"`
	}
	readJSON(r, &body)
	d := a.loadData()
	tmpl, ok := d.Templates[body.Template]
	if !ok {
		writeJSON(w, map[string]string{"error": "Template nicht gefunden"})
		return
	}

	hols := a.getAllHolidays(body.Year, d.CustomHolidays)
	planned := 0

	// Build employee→team map
	empTeam := map[string]string{}
	for _, m := range d.Mitarbeiter {
		empTeam[m.Name] = m.Team
	}

	firstDay := time.Date(body.Year, time.Month(body.Month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	for name, days := range tmpl {
		for d2 := firstDay; !d2.After(lastDay); d2 = d2.AddDate(0, 0, 1) {
			dow := int(d2.Weekday()) // 0=Sunday, 1=Monday...
			// Convert to Mon=0 format
			goW := (dow + 6) % 7 // Mon=0, Tue=1, ..., Sun=6
			shift, ok2 := days[strconv.Itoa(goW)]
			if !ok2 || shift == "" || shift == "frei" {
				continue
			}

			date := d2.Format("2006-01-02")

			// Skip holidays for the employee's team
			if hol, holExists := hols[date]; holExists && !hol.Bridge {
				team := empTeam[name]
				if team != "" && (hol.Country == team || hol.Country == "DE+IN") {
					continue
				}
			}

			slot, exists := d.Schichten[date]
			if !exists {
				slot = emptySlot()
			}

			f := slotField(&slot, shift)
			if f == nil {
				continue
			}
			if !body.Overwrite && contains(*f, name) {
				continue
			}
			if !contains(*f, name) {
				*f = append(*f, name)
				planned++
			}
			d.Schichten[date] = slot
		}
	}

	a.saveData(d)
	writeJSON(w, map[string]interface{}{"ok": true, "planned": planned})
}

// ── /api/ruf_kw ──────────────────────────────────────────────────────────────

func (a *App) handleGetRufKW(w http.ResponseWriter, r *http.Request) {
	d := a.loadData()
	writeJSON(w, d.RufKW)
}

func (a *App) handleSaveRufKW(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// The frontend posts {"ruf_kw": {"2026-W12": [...]}}. Store the inner map,
	// otherwise the KW keys sit one level too deep and apply/reload lose them.
	d := a.loadData()
	d.RufKW = unwrapRufKW(body)
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}
func (a *App) handleApplyRufKW(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year      int  `json:"year"`
		Month     int  `json:"month"`
		Overwrite bool `json:"overwrite"`
	}
	readJSON(r, &body)
	d := a.loadData()
	applied := 0

	// Iterate over all days in year (or month)
	start := time.Date(body.Year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(body.Year+1, 1, 1, 0, 0, 0, 0, time.UTC)
	if body.Month > 0 {
		start = time.Date(body.Year, time.Month(body.Month), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)
	}

	for cur := start; cur.Before(end); cur = cur.AddDate(0, 0, 1) {
		kwKey := isoWeekKey(cur)
		raw, ok := d.RufKW[kwKey]
		if !ok {
			continue
		}

		var names []string
		switch v := raw.(type) {
		case string:
			names = []string{v}
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					names = append(names, s)
				}
			}
		}

		date := cur.Format("2006-01-02")
		slot, exists := d.Schichten[date]
		if !exists {
			slot = emptySlot()
		}
		for _, name := range names {
			if !body.Overwrite && contains(slot.Rufbereitschaft, name) {
				continue
			}
			if !contains(slot.Rufbereitschaft, name) {
				slot.Rufbereitschaft = append(slot.Rufbereitschaft, name)
				applied++
			}
		}
		d.Schichten[date] = slot
	}

	a.saveData(d)
	writeJSON(w, map[string]interface{}{"ok": true, "applied": applied})
}

// ISO week key helper
func isoWeekKey(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// ── /api/snapshot ─────────────────────────────────────────────────────────────

func (a *App) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Schichten map[string]DaySlot `json:"schichten"`
	}
	readJSON(r, &body)
	d := a.loadData()
	if body.Schichten != nil {
		d.Schichten = body.Schichten
	}
	a.saveData(d)
	writeJSON(w, map[string]bool{"ok": true})
}

// ── /api/datadir ──────────────────────────────────────────────────────────────

func (a *App) handleGetDatadir(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"folder": a.dataFolder,
		"file":   a.dataFilePath(),
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
	a.dataFolder = folder
	cfg := loadConfig()
	cfg.DataFolder = folder
	saveConfig(cfg)
	writeJSON(w, map[string]interface{}{"ok": true, "folder": folder, "file": a.dataFilePath()})
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
	a.dataFolder = body.Folder
	cfg := loadConfig()
	cfg.DataFolder = body.Folder
	saveConfig(cfg)
	writeJSON(w, map[string]interface{}{"ok": true, "folder": body.Folder, "file": a.dataFilePath()})
}

// ── /api/export_data / import_data ───────────────────────────────────────────

func (a *App) handleExportData(w http.ResponseWriter, r *http.Request) {
	d := a.loadData()
	raw, _ := json.MarshalIndent(d, "", "  ")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="schichtplan_backup.json"`)
	w.Write(raw)
}

func (a *App) handleImportData(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, map[string]string{"error": "Keine Datei"})
		return
	}
	defer file.Close()

	var d AppData
	if err := json.NewDecoder(file).Decode(&d); err != nil {
		writeJSON(w, map[string]string{"error": "Ungültiges JSON: " + err.Error()})
		return
	}

	normalizeData(&d)

	a.saveData(d)
	writeJSON(w, map[string]interface{}{
		"ok":          true,
		"mitarbeiter": len(d.Mitarbeiter),
		"tage":        len(d.Schichten),
		"schichten":   len(d.Schichten),
		"templates":   len(d.Templates),
	})
}

// ── /api/export_ics / import_ics are in ics.go ───────────────────────────────
