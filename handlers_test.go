package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// newTestApp returns an App backed by a throwaway database.
func newTestApp(t *testing.T) *App {
	t.Helper()
	a := &App{}
	if err := a.setDataFolder(t.TempDir()); err != nil {
		t.Fatalf("Store öffnen: %v", err)
	}
	t.Cleanup(func() { a.store.Close() })
	return a
}

// call runs one request through the router and decodes the JSON response.
func call(t *testing.T, a *App, method, path, body string) map[string]interface{} {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("%s %s: status %d, body %s", method, path, w.Code, w.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: bad JSON %q: %v", method, path, w.Body.String(), err)
	}
	if msg, bad := out["error"]; bad {
		t.Fatalf("%s %s: %v", method, path, msg)
	}
	return out
}

// entered returns the names on one date and shift. A date without any entries
// is simply absent from the plan, which counts as empty.
func entered(t *testing.T, a *App, date, shift string) []string {
	t.Helper()
	data := call(t, a, http.MethodGet, "/api/data", "")
	days, _ := data["schichten"].(map[string]interface{})
	day, _ := days[date].(map[string]interface{})
	list, _ := day[shift].([]interface{})
	out := []string{}
	for _, v := range list {
		out = append(out, v.(string))
	}
	return out
}

// result picks one date out of a /api/schicht response.
func result(t *testing.T, res map[string]interface{}, date string) map[string]interface{} {
	t.Helper()
	day, ok := res["results"].(map[string]interface{})[date].(map[string]interface{})
	if !ok {
		t.Fatalf("kein Ergebnis für %s: %#v", date, res["results"])
	}
	return day
}

func addEmployee(t *testing.T, a *App, name, team string) {
	t.Helper()
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"`+name+`","team":"`+team+`"}`)
}

func addShift(t *testing.T, a *App, date, shift, name string) {
	t.Helper()
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["`+date+`"],"schicht":"`+shift+`","name":"`+name+`","action":"add"}`)
}

// ── Frontend contracts ────────────────────────────────────────────────────────

func TestRufKWRoundTrip(t *testing.T) {
	a := newTestApp(t)

	// The frontend posts the plan wrapped in a "ruf_kw" envelope.
	call(t, a, http.MethodPost, "/api/ruf_kw", `{"ruf_kw":{"2026-W02":["Anna"]}}`)

	got := call(t, a, http.MethodGet, "/api/data", "")
	plan, ok := got["ruf_kw"].(map[string]interface{})
	if !ok {
		t.Fatalf("ruf_kw missing or wrong type: %#v", got["ruf_kw"])
	}
	if _, ok := plan["2026-W02"]; !ok {
		t.Fatalf("KW key lost, plan is %#v", plan)
	}

	// Applying the plan must reach the individual days of that week.
	res := call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026,"month":1}`)
	if res["applied"].(float64) != 7 {
		t.Fatalf("expected 7 applied days, got %v", res["applied"])
	}
}

func TestDeleteAndRestoreEmployee(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-01", "frueh", "Anna")

	del := call(t, a, http.MethodDelete, "/api/mitarbeiter/Anna", "")
	backup, ok := del["backup"].(map[string]interface{})
	if !ok || backup["2026-04-01"] == nil {
		t.Fatalf("delete did not return a usable backup: %#v", del["backup"])
	}
	// The shift entry must be gone from the stored data, not just from the UI.
	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 0 {
		t.Fatalf("deleted employee still in shift: %#v", got)
	}

	// Re-adding plus restore brings the entry back and keeps the employee list.
	addEmployee(t, a, "Anna", "DE")
	call(t, a, http.MethodPost, "/api/mitarbeiter/restore",
		`{"name":"Anna","entries":{"2026-04-01":{"frueh":true}}}`)

	data := call(t, a, http.MethodGet, "/api/data", "")
	if n := len(data["mitarbeiter"].([]interface{})); n != 1 {
		t.Fatalf("restore clobbered the employee list, %d left", n)
	}
	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 1 {
		t.Fatalf("entry not restored: %#v", got)
	}
}

func TestRenameEmployeeUpdatesReferences(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-01", "frueh", "Anna")
	call(t, a, http.MethodPost, "/api/templates", `{"name":"Standard","template":{"Anna":{"0":"frueh"}}}`)
	call(t, a, http.MethodPost, "/api/ruf_kw", `{"ruf_kw":{"2026-W02":["Anna"]}}`)

	call(t, a, http.MethodPut, "/api/mitarbeiter/Anna", `{"name":"Anna Neu","team":"DE"}`)

	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 1 || got[0] != "Anna Neu" {
		t.Errorf("shift not renamed: %#v", got)
	}
	data := call(t, a, http.MethodGet, "/api/data", "")
	tmpl := data["templates"].(map[string]interface{})["Standard"].(map[string]interface{})
	if _, ok := tmpl["Anna Neu"]; !ok {
		t.Errorf("template not renamed: %#v", tmpl)
	}
	kw := data["ruf_kw"].(map[string]interface{})["2026-W02"].([]interface{})
	if kw[0] != "Anna Neu" {
		t.Errorf("KW plan not renamed: %#v", kw)
	}
}

func TestSchichtIgnoresMalformedDates(t *testing.T) {
	a := newTestApp(t)
	// A short date must not panic the handler.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["","2026-04-01"],"schicht":"frueh","name":"Anna","action":"add"}`)
}

func TestICSRoundTrip(t *testing.T) {
	a := newTestApp(t)
	addShift(t, a, "2026-04-01", "spaet", "Anna")

	r := httptest.NewRequest(http.MethodGet, "/api/export_ics?year=2026&month=4", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), "SUMMARY:Anna – Spätschicht") {
		t.Fatalf("event missing from export:\n%s", w.Body.String())
	}
}

// ── Conflict rules ────────────────────────────────────────────────────────────

func TestToggleUsesTheSameConflictRules(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	// 1. Mai 2026 is a public holiday in DE, so a toggle must ask first -
	// exactly like an add does.
	res := call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"toggle"}`)
	if got := result(t, res, "2026-05-01")["error"]; got != "holiday_conflict" {
		t.Fatalf("toggle skipped the holiday check: %v", got)
	}

	// 2. Forced toggle enters the shift and reports the warning.
	res = call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"toggle","force":true}`)
	if len(res["hol_warnings"].([]interface{})) != 1 {
		t.Fatalf("expected a holiday warning, got %#v", res["hol_warnings"])
	}

	// 3. Toggling again removes it.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"toggle","force":true}`)
	if got := entered(t, a, "2026-05-01", "frueh"); len(got) != 0 {
		t.Fatalf("second toggle did not remove the entry: %#v", got)
	}
}

func TestNeedsConfirmBeforeReplacingAWorkShift(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-02", "frueh", "Anna")

	res := call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"spaet","name":"Anna","action":"add"}`)
	if got := result(t, res, "2026-04-02")["error"]; got != "needs_confirm" {
		t.Fatalf("expected needs_confirm, got %v", got)
	}

	// Rufbereitschaft is never reported as the blocking shift: it may run
	// alongside a work shift and survives a replacement.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"rufbereitschaft","name":"Anna","action":"add","force":true}`)
	res = call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"spaet","name":"Anna","action":"add"}`)
	blocking := result(t, res, "2026-04-02")["blocking"].([]interface{})
	if len(blocking) != 1 || blocking[0] != "frueh" {
		t.Fatalf("expected only frueh to block, got %#v", blocking)
	}
}

func TestConfirmedReplaceGivesUpTheOldShift(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-02", "frueh", "Anna")
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"rufbereitschaft","name":"Anna","action":"add","force":true}`)

	// The user confirmed "Schicht ersetzen?", so Früh gives way to Spät -
	// while Rufbereitschaft stays, as the dialog promises.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"spaet","name":"Anna","action":"add","force":true,"replace":true}`)

	if got := entered(t, a, "2026-04-02", "frueh"); len(got) != 0 {
		t.Errorf("Frühschicht was not given up: %#v", got)
	}
	if got := entered(t, a, "2026-04-02", "spaet"); len(got) != 1 {
		t.Errorf("Spätschicht not entered: %#v", got)
	}
	if got := entered(t, a, "2026-04-02", "rufbereitschaft"); len(got) != 1 {
		t.Errorf("Rufbereitschaft should have stayed: %#v", got)
	}
}

func TestForcedHolidayEntryKeepsOtherShifts(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	// 1. Mai is a DE holiday, so the first entry needs the holiday confirmation.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"add","force":true}`)

	// Confirming only the holiday dialog must not silently drop another shift.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"spaet","name":"Anna","action":"add","force":true}`)
	if got := entered(t, a, "2026-05-01", "frueh"); len(got) != 1 {
		t.Fatalf("Frühschicht was dropped without a replace confirmation: %#v", got)
	}
}

// ── Storage ───────────────────────────────────────────────────────────────────

func TestLegacyJSONIsImportedOnce(t *testing.T) {
	folder := t.TempDir()
	legacy := `{"mitarbeiter":[{"name":"Alt","team":"DE","color":"#fff","prefs":{}}],
		"schichten":{"2026-03-02":{"frueh":["Alt"]}},"notizen":{"2026-03-02":"Notiz"},
		"soll":{"frueh":2,"spaet":1,"rufbereitschaft":1},"templates":{},"ruf_kw":{}}`
	if err := os.WriteFile(filepath.Join(folder, dataFileName), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := openStore(folder)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	d, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(d.Mitarbeiter) != 1 || d.Mitarbeiter[0].Name != "Alt" {
		t.Fatalf("employee not imported: %#v", d.Mitarbeiter)
	}
	if d.Notizen["2026-03-02"] != "Notiz" || d.Soll.Frueh != 2 {
		t.Fatalf("notes/soll not imported: %#v %#v", d.Notizen, d.Soll)
	}

	// A second start must not import the JSON again over newer edits.
	if _, _, err := s.DeleteEmployee("Alt"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s2, err := openStore(folder)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	d, err = s2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Mitarbeiter) != 0 {
		t.Fatalf("legacy JSON was imported a second time: %#v", d.Mitarbeiter)
	}
}

func TestChangesAreRecordedInHistory(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-01", "frueh", "Anna")

	r := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	var entries []ChangeEntry
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatalf("history: %v (%s)", err, w.Body.String())
	}
	if len(entries) < 2 {
		t.Fatalf("expected the employee and the shift to be logged, got %#v", entries)
	}
	if entries[0].Action != "schicht:add" {
		t.Errorf("newest entry should be the shift, got %q", entries[0].Action)
	}
}

func TestBackupExportAndImport(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-01", "frueh", "Anna")

	r := httptest.NewRequest(http.MethodGet, "/api/export_data", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	backup := w.Body.String()

	// Wiping and re-importing has to restore the same plan.
	call(t, a, http.MethodDelete, "/api/mitarbeiter/Anna", "")

	body, ctype := multipartBody(t, "backup.json", backup)
	r = httptest.NewRequest(http.MethodPost, "/api/import_data", strings.NewReader(body))
	r.Header.Set("Content-Type", ctype)
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)

	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 1 || got[0] != "Anna" {
		t.Fatalf("plan not restored from backup: %#v (%s)", got, w.Body.String())
	}
}

// multipartBody builds a minimal multipart body with one "file" part.
func multipartBody(t *testing.T, filename, content string) (string, string) {
	t.Helper()
	const boundary = "TESTBOUNDARY"
	body := "--" + boundary + "\r\n" +
		`Content-Disposition: form-data; name="file"; filename="` + filename + `"` + "\r\n" +
		"Content-Type: application/json\r\n\r\n" + content + "\r\n--" + boundary + "--\r\n"
	return body, "multipart/form-data; boundary=" + boundary
}

// ── Feiertage ─────────────────────────────────────────────────────────────────

func TestCustomHolidayAsksLikeAStatutoryOne(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Ravi", "IN")
	call(t, a, http.MethodPost, "/api/custom_holidays",
		`{"date":"2026-07-15","name":"Betriebsausflug","country":"IN"}`)

	res := call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-07-15"],"schicht":"frueh","name":"Ravi","action":"add"}`)
	day := result(t, res, "2026-07-15")
	if day["error"] != "holiday_conflict" || day["holiday"] != "Betriebsausflug" {
		t.Fatalf("own holiday did not ask: %#v", day)
	}

	// A holiday of the other team leaves the entry alone.
	addEmployee(t, a, "Anna", "DE")
	res = call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-07-15"],"schicht":"frueh","name":"Anna","action":"add"}`)
	if got := result(t, res, "2026-07-15")["error"]; got != nil {
		t.Fatalf("holiday of the other team should not block: %v", got)
	}

	// Confirming enters it anyway.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-07-15"],"schicht":"frueh","name":"Ravi","action":"add","force":true}`)
	if got := entered(t, a, "2026-07-15", "frueh"); len(got) != 2 {
		t.Fatalf("forced entry missing: %#v", got)
	}
}

func TestMovableIndianHolidaysAreTabulated(t *testing.T) {
	a := newTestApp(t)

	// Inside the tabulated range Holi and Diwali are known...
	hols := call(t, a, http.MethodGet, "/api/holidays/2029", "")
	if h, ok := hols["2029-03-01"].(map[string]interface{}); !ok || h["name"] != "Holi" {
		t.Errorf("Holi 2029 missing: %#v", hols["2029-03-01"])
	}
	if h, ok := hols["2029-11-05"].(map[string]interface{}); !ok || h["name"] != "Diwali" {
		t.Errorf("Diwali 2029 missing: %#v", hols["2029-11-05"])
	}

	// ...beyond it they are absent, and the API says where the table ends so
	// the UI can ask for them to be entered by hand.
	cover := call(t, a, http.MethodGet, "/api/holiday_coverage", "")
	last := int(cover["in_movable_to"].(float64))
	if last != inMovableLastYear {
		t.Fatalf("coverage %d does not match the table (%d)", last, inMovableLastYear)
	}
	beyond := call(t, a, http.MethodGet, "/api/holidays/"+strconv.Itoa(last+1), "")
	for date, h := range beyond {
		name := h.(map[string]interface{})["name"]
		if name == "Holi" || name == "Diwali" {
			t.Errorf("unexpected %v on %s beyond the table", name, date)
		}
	}
	// The fixed-date Indian holidays are still there.
	if _, ok := beyond[strconv.Itoa(last+1)+"-01-26"]; !ok {
		t.Error("Republic Day missing beyond the table")
	}
}

func TestHoliAndDiwaliTablesCoverTheSameYears(t *testing.T) {
	for year := inMovableFirstYear; year <= inMovableLastYear; year++ {
		if _, ok := holiDates[year]; !ok {
			t.Errorf("Holi %d fehlt", year)
		}
		if _, ok := diwaliDates[year]; !ok {
			t.Errorf("Diwali %d fehlt", year)
		}
	}
	if len(holiDates) != inMovableLastYear-inMovableFirstYear+1 {
		t.Errorf("Holi-Tabelle hat %d Einträge, erwartet %d",
			len(holiDates), inMovableLastYear-inMovableFirstYear+1)
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

func TestFailedFolderSwitchKeepsTheOldOne(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	good := a.dataFolder

	// A folder whose database path is occupied by a directory cannot be opened.
	broken := t.TempDir()
	if err := os.Mkdir(filepath.Join(broken, dbFileName), 0755); err != nil {
		t.Fatal(err)
	}
	if err := a.setDataFolder(broken); err == nil {
		t.Fatal("expected the broken folder to be refused")
	}

	// The app has to keep working on the folder it had.
	if a.dataFolder != good || a.store == nil {
		t.Fatalf("folder switch tore down the working store: %q, store=%v", a.dataFolder, a.store != nil)
	}
	data := call(t, a, http.MethodGet, "/api/data", "")
	if n := len(data["mitarbeiter"].([]interface{})); n != 1 {
		t.Fatalf("data no longer reachable, %d Mitarbeiter", n)
	}
}

func TestDeleteReturnsTheEmployeeForRestore(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	call(t, a, http.MethodPut, "/api/mitarbeiter/Anna",
		`{"name":"Anna","team":"DE","color":"#123456","icon":"🌙"}`)

	del := call(t, a, http.MethodDelete, "/api/mitarbeiter/Anna", "")
	emp, ok := del["employee"].(map[string]interface{})
	if !ok {
		t.Fatalf("delete did not return the employee: %#v", del["employee"])
	}
	if emp["team"] != "DE" || emp["color"] != "#123456" || emp["icon"] != "🌙" {
		t.Fatalf("employee record incomplete: %#v", emp)
	}
}

func TestAutoplanSkipsShiftConflicts(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	// 2026-06-01 is a Monday; Anna already works the early shift there.
	addShift(t, a, "2026-06-01", "frueh", "Anna")
	call(t, a, http.MethodPost, "/api/templates", `{"name":"Spaet","template":{"Anna":{"0":"spaet"}}}`)

	res := call(t, a, http.MethodPost, "/api/autoplan", `{"year":2026,"month":6,"template":"Spaet"}`)
	if res["skipped_conflict"].(float64) != 1 {
		t.Fatalf("expected one skipped Monday, got %v", res["skipped_conflict"])
	}
	if res["planned"].(float64) != 4 {
		t.Fatalf("expected the other four Mondays to be planned, got %v", res["planned"])
	}
	// The existing early shift must be untouched, and no double booking.
	if got := entered(t, a, "2026-06-01", "frueh"); len(got) != 1 {
		t.Errorf("existing shift changed: %#v", got)
	}
	if got := entered(t, a, "2026-06-01", "spaet"); len(got) != 0 {
		t.Errorf("autoplan double-booked the day: %#v", got)
	}
}
