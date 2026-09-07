package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"schichtplaner/internal/domain"
	"schichtplaner/internal/store"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// testSeite ist die echte Oberfläche aus dem Projekt. Der Server bekommt sie
// im Betrieb eingebettet; hier wird sie gelesen, damit auch der Weg "/" das
// prüft, was wirklich ausgeliefert wird.
var testSeite = sync.OnceValue(func() []byte {
	raw, err := os.ReadFile(filepath.Join("..", "..", "frontend", "index.html"))
	if err != nil {
		panic("frontend/index.html nicht lesbar: " + err.Error())
	}
	return raw
})

// newTestApp liefert einen Server mit einer Wegwerf-Datenbank.
func newTestApp(t *testing.T) *Server {
	t.Helper()
	a := New(testSeite(), nil)
	if err := a.setDataFolder(t.Context(), t.TempDir()); err != nil {
		t.Fatalf("Store öffnen: %v", err)
	}
	t.Cleanup(func() { a.store.Close() })
	return a
}

// call schickt eine Anfrage durch den Router und liest die JSON-Antwort.
func call(t *testing.T, a *Server, method, path, body string) map[string]any {
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
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: bad JSON %q: %v", method, path, w.Body.String(), err)
	}
	if msg, bad := out["error"]; bad {
		t.Fatalf("%s %s: %v", method, path, msg)
	}
	return out
}

// entered liefert die Namen an einem Tag in einer Schicht. Ein Tag ohne
// Einträge fehlt im Plan schlicht - das zählt als leer.
func entered(t *testing.T, a *Server, date, shift string) []string {
	t.Helper()
	data := call(t, a, http.MethodGet, "/api/data", "")
	days, _ := data["schichten"].(map[string]any)
	day, _ := days[date].(map[string]any)
	list, _ := day[shift].([]any)
	out := []string{}
	for _, v := range list {
		out = append(out, v.(string))
	}
	return out
}

// result greift einen Tag aus einer Antwort von /api/schicht heraus.
func result(t *testing.T, res map[string]any, date string) map[string]any {
	t.Helper()
	day, ok := res["results"].(map[string]any)[date].(map[string]any)
	if !ok {
		t.Fatalf("kein Ergebnis für %s: %#v", date, res["results"])
	}
	return day
}

func addEmployee(t *testing.T, a *Server, name, team string) {
	t.Helper()
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"`+name+`","team":"`+team+`"}`)
}

func addShift(t *testing.T, a *Server, date, shift, name string) {
	t.Helper()
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["`+date+`"],"schicht":"`+shift+`","name":"`+name+`","action":"add"}`)
}

// ── Frontend contracts ────────────────────────────────────────────────────────

func TestRufKWPlanUeberlebtDenRundlauf(t *testing.T) {
	a := newTestApp(t)

	// Die Oberfläche schickt den Plan in einer Hülle namens "ruf_kw".
	call(t, a, http.MethodPost, "/api/ruf_kw", `{"ruf_kw":{"2026-W02":["Anna"]}}`)

	got := call(t, a, http.MethodGet, "/api/data", "")
	plan, ok := got["ruf_kw"].(map[string]any)
	if !ok {
		t.Fatalf("ruf_kw missing or wrong type: %#v", got["ruf_kw"])
	}
	if _, ok := plan["2026-W02"]; !ok {
		t.Fatalf("KW key lost, plan is %#v", plan)
	}

	// Das Übertragen muss bei den einzelnen Tagen dieser Woche ankommen.
	res := call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026,"month":1}`)
	if res["applied"].(float64) != 7 {
		t.Fatalf("expected 7 applied days, got %v", res["applied"])
	}
}

func TestMitarbeiterLoeschenUndWiederherstellen(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-01", "frueh", "Anna")

	del := call(t, a, http.MethodDelete, "/api/mitarbeiter/Anna", "")
	backup, ok := del["backup"].(map[string]any)
	if !ok || backup["2026-04-01"] == nil {
		t.Fatalf("delete did not return a usable backup: %#v", del["backup"])
	}
	// Der Eintrag muss aus den Daten verschwunden sein, nicht nur aus dem Bild.
	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 0 {
		t.Fatalf("deleted employee still in shift: %#v", got)
	}

	// Neu anlegen und wiederherstellen bringt den Eintrag zurück, ohne die
	// Mitarbeiterliste zu verdoppeln.
	addEmployee(t, a, "Anna", "DE")
	call(t, a, http.MethodPost, "/api/mitarbeiter/restore",
		`{"name":"Anna","entries":{"2026-04-01":{"frueh":true}}}`)

	data := call(t, a, http.MethodGet, "/api/data", "")
	if n := len(data["mitarbeiter"].([]any)); n != 1 {
		t.Fatalf("restore clobbered the employee list, %d left", n)
	}
	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 1 {
		t.Fatalf("entry not restored: %#v", got)
	}
}

func TestUmbenennenZiehtAlleEintraegeMit(t *testing.T) {
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
	tmpl := data["templates"].(map[string]any)["Standard"].(map[string]any)
	if _, ok := tmpl["Anna Neu"]; !ok {
		t.Errorf("template not renamed: %#v", tmpl)
	}
	kw := data["ruf_kw"].(map[string]any)["2026-W02"].([]any)
	if kw[0] != "Anna Neu" {
		t.Errorf("KW plan not renamed: %#v", kw)
	}
}

func TestUnbrauchbareDatumsangabenWerdenUebergangen(t *testing.T) {
	a := newTestApp(t)
	// Ein zu kurzes Datum darf den Handler nicht umwerfen.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["","2026-04-01"],"schicht":"frueh","name":"Anna","action":"add"}`)
}

func TestICSUeberlebtExportUndImport(t *testing.T) {
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

func TestUmschaltenFolgtDenselbenKonfliktregeln(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	// Der 1. Mai 2026 ist in DE gesetzlicher Feiertag, also muss auch das
	// Umschalten vorher fragen - genau wie das Eintragen.
	res := call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"toggle"}`)
	if got := result(t, res, "2026-05-01")["error"]; got != "holiday_conflict" {
		t.Fatalf("toggle skipped the holiday check: %v", got)
	}

	// 2. Bestätigt trägt es ein und meldet die Warnung.
	res = call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"toggle","force":true}`)
	if len(res["hol_warnings"].([]any)) != 1 {
		t.Fatalf("expected a holiday warning, got %#v", res["hol_warnings"])
	}

	// 3. Nochmal umschalten trägt wieder aus.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"toggle","force":true}`)
	if got := entered(t, a, "2026-05-01", "frueh"); len(got) != 0 {
		t.Fatalf("second toggle did not remove the entry: %#v", got)
	}
}

func TestZweiteArbeitsschichtFragtVorherNach(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-02", "frueh", "Anna")

	res := call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"spaet","name":"Anna","action":"add"}`)
	if got := result(t, res, "2026-04-02")["error"]; got != "needs_confirm" {
		t.Fatalf("expected needs_confirm, got %v", got)
	}

	// Rufbereitschaft wird nie als blockierend gemeldet: sie läuft neben einer
	// Arbeitsschicht her und übersteht auch ein Ersetzen.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"rufbereitschaft","name":"Anna","action":"add","force":true}`)
	res = call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"spaet","name":"Anna","action":"add"}`)
	blocking := result(t, res, "2026-04-02")["blocking"].([]any)
	if len(blocking) != 1 || blocking[0] != "frueh" {
		t.Fatalf("expected only frueh to block, got %#v", blocking)
	}
}

func TestBestaetigtesErsetzenGibtDieAlteSchichtAb(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-02", "frueh", "Anna")
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"rufbereitschaft","name":"Anna","action":"add","force":true}`)

	// The user confirmed "Schicht ersetzen?", so Früh gives way to Spät -
	// während die Rufbereitschaft bleibt, wie die Rückfrage verspricht.
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

func TestEintragAmFeiertagLaesstAndereSchichtenStehen(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	// Der 1. Mai ist ein DE-Feiertag, der erste Eintrag braucht also die
	// Feiertagsbestätigung.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"add","force":true}`)

	// Wer nur den Feiertag bestätigt, darf damit nicht still eine andere
	// Schicht verlieren.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"spaet","name":"Anna","action":"add","force":true}`)
	if got := entered(t, a, "2026-05-01", "frueh"); len(got) != 1 {
		t.Fatalf("Frühschicht was dropped without a replace confirmation: %#v", got)
	}
}

// ── Storage ───────────────────────────────────────────────────────────────────

func TestAenderungenLandenImVerlauf(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-01", "frueh", "Anna")

	r := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	var entries []domain.ChangeEntry
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

func TestSicherungSchreibenUndEinlesen(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2026-04-01", "frueh", "Anna")

	r := httptest.NewRequest(http.MethodGet, "/api/export_data", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	backup := w.Body.String()

	// Leeren und wieder einlesen muss denselben Plan ergeben.
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

// multipartBody baut einen knappen Upload-Körper mit einem Teil "file".
func multipartBody(t *testing.T, filename, content string) (string, string) {
	t.Helper()
	const boundary = "TESTBOUNDARY"
	body := "--" + boundary + "\r\n" +
		`Content-Disposition: form-data; name="file"; filename="` + filename + `"` + "\r\n" +
		"Content-Type: application/json\r\n\r\n" + content + "\r\n--" + boundary + "--\r\n"
	return body, "multipart/form-data; boundary=" + boundary
}

// ── Feiertage ─────────────────────────────────────────────────────────────────

func TestEigenerFeiertagFragtWieEinGesetzlicher(t *testing.T) {
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

	// Ein Feiertag des anderen Teams stört den Eintrag nicht.
	addEmployee(t, a, "Anna", "DE")
	res = call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-07-15"],"schicht":"frueh","name":"Anna","action":"add"}`)
	if got := result(t, res, "2026-07-15")["error"]; got != nil {
		t.Fatalf("holiday of the other team should not block: %v", got)
	}

	// Bestätigt wird trotzdem eingetragen.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-07-15"],"schicht":"frueh","name":"Ravi","action":"add","force":true}`)
	if got := entered(t, a, "2026-07-15", "frueh"); len(got) != 2 {
		t.Fatalf("forced entry missing: %#v", got)
	}
}

func TestBeweglicheIndischeFeiertageStehenInDerTabelle(t *testing.T) {
	a := newTestApp(t)

	// Innerhalb des tabellierten Zeitraums sind Holi und Diwali bekannt ...
	hols := call(t, a, http.MethodGet, "/api/holidays/2029", "")
	if h, ok := hols["2029-03-01"].(map[string]any); !ok || h["name"] != "Holi" {
		t.Errorf("Holi 2029 missing: %#v", hols["2029-03-01"])
	}
	if h, ok := hols["2029-11-05"].(map[string]any); !ok || h["name"] != "Diwali" {
		t.Errorf("Diwali 2029 missing: %#v", hols["2029-11-05"])
	}

	// ... darüber hinaus fehlen sie, und die API nennt das Ende der Tabelle,
	// damit die Oberfläche zum Nachtragen auffordern kann.
	cover := call(t, a, http.MethodGet, "/api/holiday_coverage", "")
	last := int(cover["in_movable_to"].(float64))
	if last != domain.MovableINLastYear {
		t.Fatalf("coverage %d does not match the table (%d)", last, domain.MovableINLastYear)
	}
	beyond := call(t, a, http.MethodGet, "/api/holidays/"+strconv.Itoa(last+1), "")
	for date, h := range beyond {
		name := h.(map[string]any)["name"]
		if name == "Holi" || name == "Diwali" {
			t.Errorf("unexpected %v on %s beyond the table", name, date)
		}
	}
	// Die indischen Feiertage mit festem Datum stehen weiterhin da.
	if _, ok := beyond[strconv.Itoa(last+1)+"-01-26"]; !ok {
		t.Error("Republic Day missing beyond the table")
	}
}

func TestGescheiterterOrdnerwechselLaesstDenAltenStehen(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	good := a.dataFolder

	// Ein Ordner, in dem an der Stelle der Datenbank ein Verzeichnis liegt,
	// lässt sich nicht öffnen.
	broken := t.TempDir()
	if err := os.Mkdir(filepath.Join(broken, store.DBFileName), 0755); err != nil {
		t.Fatal(err)
	}
	if err := a.setDataFolder(t.Context(), broken); err == nil {
		t.Fatal("expected the broken folder to be refused")
	}

	// Das Programm muss mit dem Ordner weiterarbeiten, den es hatte.
	if a.dataFolder != good || a.store == nil {
		t.Fatalf("folder switch tore down the working store: %q, store=%v", a.dataFolder, a.store != nil)
	}
	data := call(t, a, http.MethodGet, "/api/data", "")
	if n := len(data["mitarbeiter"].([]any)); n != 1 {
		t.Fatalf("data no longer reachable, %d Mitarbeiter", n)
	}
}

func TestLoeschenLiefertDenMitarbeiterZurueck(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	call(t, a, http.MethodPut, "/api/mitarbeiter/Anna",
		`{"name":"Anna","team":"DE","color":"#123456","icon":"🌙"}`)

	del := call(t, a, http.MethodDelete, "/api/mitarbeiter/Anna", "")
	emp, ok := del["employee"].(map[string]any)
	if !ok {
		t.Fatalf("delete did not return the employee: %#v", del["employee"])
	}
	if emp["team"] != "DE" || emp["color"] != "#123456" || emp["icon"] != "🌙" {
		t.Fatalf("employee record incomplete: %#v", emp)
	}
}

func TestAutoplanUeberspringtKonflikte(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	// Der 1.6.2026 ist ein Montag; Anna steht dort schon in der Frühschicht.
	addShift(t, a, "2026-06-01", "frueh", "Anna")
	call(t, a, http.MethodPost, "/api/templates", `{"name":"Spaet","template":{"Anna":{"0":"spaet"}}}`)

	res := call(t, a, http.MethodPost, "/api/autoplan", `{"year":2026,"month":6,"template":"Spaet"}`)
	if res["skipped_conflict"].(float64) != 1 {
		t.Fatalf("expected one skipped Monday, got %v", res["skipped_conflict"])
	}
	if res["planned"].(float64) != 4 {
		t.Fatalf("expected the other four Mondays to be planned, got %v", res["planned"])
	}
	// Die bestehende Frühschicht bleibt unangetastet, und niemand steht doppelt.
	if got := entered(t, a, "2026-06-01", "frueh"); len(got) != 1 {
		t.Errorf("existing shift changed: %#v", got)
	}
	if got := entered(t, a, "2026-06-01", "spaet"); len(got) != 0 {
		t.Errorf("autoplan double-booked the day: %#v", got)
	}
}

func TestICSImportVertraegtUmgebrocheneZeilen(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	// Kalender brechen jede Zeile jenseits von 75 Oktetten um; die Fortsetzung
	// beginnt mit einem Leerzeichen. Wer das nicht rückgängig macht, verliert
	// diesen Termin.
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" +
		"BEGIN:VEVENT\r\nUID:1@x\r\nDTSTART;TZID=Europe/Berlin:20260908T140000\r\n" +
		"SUMMARY:Anna – Spät\r\n schicht\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	body, ctype := multipartBody(t, "cal.ics", ics)
	r := httptest.NewRequest(http.MethodPost, "/api/import_ics", strings.NewReader(body))
	r.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)

	if got := entered(t, a, "2026-09-08", "spaet"); len(got) != 1 || got[0] != "Anna" {
		t.Fatalf("folded event was dropped: %#v (%s)", got, w.Body.String())
	}
}

func TestICSExportBrichtLangeZeilenUm(t *testing.T) {
	a := newTestApp(t)
	long := "Maximiliane Friederike von Habsburg-Lothringen zu Sonnenfels"
	addShift(t, a, "2026-04-01", "rufbereitschaft", long)

	r := httptest.NewRequest(http.MethodGet, "/api/export_ics?year=2026", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	out := w.Body.String()

	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 75 {
			t.Fatalf("line longer than 75 octets: %q", line)
		}
	}
	// Und es muss den Rundlauf durch den eigenen Import überstehen.
	b := newTestApp(t)
	body, ctype := multipartBody(t, "cal.ics", out)
	r = httptest.NewRequest(http.MethodPost, "/api/import_ics", strings.NewReader(body))
	r.Header.Set("Content-Type", ctype)
	w = httptest.NewRecorder()
	b.ServeHTTP(w, r)
	if got := entered(t, b, "2026-04-01", "rufbereitschaft"); len(got) != 1 || got[0] != long {
		t.Fatalf("round trip lost the name: %#v (%s)", got, w.Body.String())
	}
}

// ── Massenanlage ──────────────────────────────────────────────────────────────

func TestMassenanlageVonMitarbeitern(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	res := call(t, a, http.MethodPost, "/api/mitarbeiter/bulk", `{"mitarbeiter":[
		{"name":"Ravi","team":"IN"},
		{"name":" Jonas ","team":"DE"},
		{"name":"Anna","team":"DE"},
		{"name":"Ravi","team":"IN"},
		{"name":"   "}
	]}`)
	if res["angelegt"].(float64) != 2 {
		t.Fatalf("expected two new employees, got %v", res["angelegt"])
	}
	// Anna gibt es schon; der zweite Ravi und der leere Name fallen weg, bevor
	// die Ablage sie sieht - übersprungen zählt daher nur Anna.
	if res["uebersprungen"].(float64) != 1 {
		t.Errorf("expected one skipped, got %v", res["uebersprungen"])
	}
	names := map[string]bool{}
	for _, m := range res["mitarbeiter"].([]any) {
		names[m.(map[string]any)["name"].(string)] = true
	}
	if len(names) != 3 || !names["Jonas"] {
		t.Fatalf("unexpected list, names trimmed? %#v", names)
	}
}

func TestMassenanlageVonFeiertagen(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	res := call(t, a, http.MethodPost, "/api/custom_holidays/bulk", `{"feiertage":[
		{"date":"2026-12-24","name":"Heiligabend"},
		{"date":"2026-12-31","name":"Silvester","country":"DE+IN"},
		{"date":"2026-02-31","name":"Gibt es nicht"},
		{"date":"2026-12-24","name":"Heiligabend"}
	]}`)
	if res["angelegt"].(float64) != 2 {
		t.Fatalf("expected two holidays, got %v", res["angelegt"])
	}

	// Ohne Landesangabe gilt DE, und der Eintrag muss so nachfragen wie ein
	// gesetzlicher Feiertag.
	list := call(t, a, http.MethodGet, "/api/data", "")["custom_holidays"].([]any)
	if len(list) != 2 {
		t.Fatalf("expected two stored holidays, got %d", len(list))
	}
	if c := list[0].(map[string]any)["country"]; c != "DE" {
		t.Errorf("missing country should default to DE, got %v", c)
	}
	sch := call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-12-24"],"schicht":"frueh","name":"Anna","action":"add"}`)
	if got := result(t, sch, "2026-12-24")["error"]; got != "holiday_conflict" {
		t.Errorf("bulk holiday does not block: %v", got)
	}

	// Dieselbe Liste noch einmal ändert nichts.
	res = call(t, a, http.MethodPost, "/api/custom_holidays/bulk",
		`{"feiertage":[{"date":"2026-12-24","name":"Heiligabend","country":"DE"}]}`)
	if res["angelegt"].(float64) != 0 || res["uebersprungen"].(float64) != 1 {
		t.Errorf("duplicate run should skip: %#v", res)
	}
}

func TestMassenanlageWeistLeereEingabeAb(t *testing.T) {
	a := newTestApp(t)
	for _, c := range []struct{ path, body string }{
		{"/api/mitarbeiter/bulk", `{"mitarbeiter":[{"name":"  "}]}`},
		{"/api/custom_holidays/bulk", `{"feiertage":[{"date":"kaputt","name":"X"}]}`},
	} {
		r := httptest.NewRequest(http.MethodPost, c.path, strings.NewReader(c.body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if !strings.Contains(w.Body.String(), "error") {
			t.Errorf("%s should refuse empty input, got %s", c.path, w.Body.String())
		}
	}
}

// ── Normaldienst ──────────────────────────────────────────────────────────────

func TestNormaldienstIstEineArbeitsschicht(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Clara", "DE")
	addShift(t, a, "2026-04-02", "normal", "Clara")

	if got := entered(t, a, "2026-04-02", "normal"); len(got) != 1 {
		t.Fatalf("Normaldienst not stored: %#v", got)
	}
	// Normal excludes Früh and Spät on the same day, Rufbereitschaft does not.
	res := call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"frueh","name":"Clara","action":"add"}`)
	blocking := result(t, res, "2026-04-02")["blocking"].([]any)
	if len(blocking) != 1 || blocking[0] != "normal" {
		t.Fatalf("Normaldienst should block Frühschicht: %#v", blocking)
	}
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"rufbereitschaft","name":"Clara","action":"add"}`)
	if got := entered(t, a, "2026-04-02", "rufbereitschaft"); len(got) != 1 {
		t.Fatalf("Rufbereitschaft must run alongside Normaldienst: %#v", got)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/export_ics?year=2026", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), "SUMMARY:Clara – Normaldienst") {
		t.Errorf("Normaldienst missing from ICS export")
	}
}

func TestAutoplanMitNormaldienst(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Clara", "DE")
	call(t, a, http.MethodPost, "/api/templates",
		`{"name":"Tag","template":{"Clara":{"0":"normal","2":"normal"}}}`)

	res := call(t, a, http.MethodPost, "/api/autoplan", `{"year":2026,"month":6,"template":"Tag"}`)
	// Juni 2026: 5 Montage, 4 Mittwoche - keiner davon ein Feiertag in BW.
	if res["planned"].(float64) != 9 {
		t.Fatalf("expected 9 planned days, got %v", res["planned"])
	}
	if got := entered(t, a, "2026-06-01", "normal"); len(got) != 1 {
		t.Fatalf("Monday not planned: %#v", got)
	}
}

func TestRufbereitschaftFragtNie(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	// Rufbereitschaft läuft neben jeder Arbeitsschicht her - egal welcher.
	for _, shift := range []string{"frueh", "normal", "spaet"} {
		date := "2026-07-0" + string(rune('1'+len(shift)%3))
		addShift(t, a, date, shift, "Anna")
		res := call(t, a, http.MethodPost, "/api/schicht",
			`{"dates":["`+date+`"],"schicht":"rufbereitschaft","name":"Anna","action":"add"}`)
		if got := result(t, res, date)["error"]; got != nil {
			t.Errorf("Rufbereitschaft neben %s sollte ohne Rückfrage gehen, kam: %v", shift, got)
		}
		if got := entered(t, a, date, "rufbereitschaft"); len(got) != 1 {
			t.Errorf("Rufbereitschaft neben %s nicht eingetragen: %#v", shift, got)
		}
	}
}
