package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Was passiert, wenn nicht die eigene Oberfläche fragt, sondern ein Tippfehler,
// ein fremder Kalender oder eine von Hand bearbeitete Sicherung?
//
// Der Maßstab ist immer derselbe: nichts darf in der Datenbank landen, was im
// Kalender nie wieder auftaucht. Ein Tag, den es nicht gibt, und ein Name, den
// keine Mitarbeiterliste kennt, wären genau das - unsichtbar und nicht mehr
// löschbar.

// jsonVon liest eine JSON-Antwort als Karte.
func jsonVon(t *testing.T, roh []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(roh, &out); err != nil {
		t.Fatalf("keine JSON-Karte: %s", roh)
	}
	return out
}

// ── Tage, die es nicht gibt ───────────────────────────────────────────────────

func TestSchichtNimmtNurEchteTage(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	unbrauchbar := []string{
		"",            // leer
		"morgen",      // gar kein Datum
		"2026-2-1",    // ohne führende Null
		"01.02.2026",  // deutsche Schreibweise
		"2026-02-31",  // richtige Gestalt, gibt es aber nicht
		"2025-02-29",  // kein Schaltjahr
		"2026-13-01",  // Monat 13
		"0000-01-01",  // Jahr 0 - für die Zeitrechnung gültig, für einen Plan nicht
		"1800-01-01",  // vor dem planbaren Bereich
		"3000-01-01",  // dahinter
		"2026-04-01 ", // mit Leerzeichen
		"2026-04-01T00:00:00",
	}
	for _, d := range unbrauchbar {
		res := call(t, a, http.MethodPost, "/api/schicht",
			mustJSON(map[string]any{"dates": []string{d}, "schicht": "frueh", "name": "Anna", "action": "add"}))
		if ergebnisse, _ := res["results"].(map[string]any); len(ergebnisse) != 0 {
			t.Errorf("%q wurde angenommen: %#v", d, ergebnisse)
		}
	}

	// Und der Plan ist danach immer noch leer.
	data := call(t, a, http.MethodGet, "/api/data", "")
	if tage, _ := data["schichten"].(map[string]any); len(tage) != 0 {
		t.Fatalf("es ist doch etwas hängengeblieben: %#v", tage)
	}
}

func TestSchaltjahrWirdAngenommen(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	addShift(t, a, "2024-02-29", "frueh", "Anna")
	if got := entered(t, a, "2024-02-29", "frueh"); len(got) != 1 {
		t.Fatalf("29.2.2024 ist ein echter Tag: %v", got)
	}
}

func TestNotizNurAufEchtenTagen(t *testing.T) {
	a := newTestApp(t)
	w := roh(a, http.MethodPost, "/api/notiz", `{"date":"morgen","text":"x"}`)
	if !strings.Contains(w.Body.String(), "error") {
		t.Fatalf("Notiz auf Unsinnsdatum angenommen: %s", w.Body.String())
	}
	call(t, a, http.MethodPost, "/api/notiz", `{"date":"2026-04-01","text":"Übergabe"}`)
	data := call(t, a, http.MethodGet, "/api/data", "")
	notizen, _ := data["notizen"].(map[string]any)
	if notizen["2026-04-01"] != "Übergabe" {
		t.Fatalf("Notiz nicht gespeichert: %#v", notizen)
	}
}

func TestPasteNimmtNurEchteTage(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	call(t, a, http.MethodPost, "/api/paste",
		`{"slot":{"frueh":["Anna"]},"dates":["morgen","2026-02-31","2026-04-01"],"mode":"replace"}`)

	data := call(t, a, http.MethodGet, "/api/data", "")
	tage, _ := data["schichten"].(map[string]any)
	if len(tage) != 1 {
		t.Fatalf("nur der 1.4. sollte durchkommen: %#v", tage)
	}
}

// ── Namen, die es nicht gibt ──────────────────────────────────────────────────

func TestSchichtNurFuerBekannteMitarbeiter(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	for _, name := range []string{"", "   ", "Niemand"} {
		w := roh(a, http.MethodPost, "/api/schicht",
			mustJSON(map[string]any{
				"dates": []string{"2026-04-01"}, "schicht": "frueh", "name": name, "action": "add",
			}))
		if !strings.Contains(w.Body.String(), "error") {
			t.Errorf("Name %q wurde angenommen: %s", name, w.Body.String())
		}
	}
	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 0 {
		t.Fatalf("Tag ist nicht leer: %v", got)
	}
}

func TestNameWirdVorDemEintragenBeschnitten(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-01"],"schicht":"frueh","name":"  Anna  ","action":"add"}`)
	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 1 || got[0] != "Anna" {
		t.Fatalf("Leerzeichen nicht abgeschnitten: %#v", got)
	}
}

func TestSchichtLeerenBrauchtKeinenNamen(t *testing.T) {
	a := newTestApp(t)
	addShift(t, a, "2026-04-01", "frueh", "Anna")
	addShift(t, a, "2026-04-01", "frueh", "Berta")

	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-01"],"schicht":"frueh","name":"","action":"clear_shift"}`)
	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 0 {
		t.Fatalf("Zeile nicht geleert: %v", got)
	}
}

func TestWiederherstellenNurFuerEinenBestehendenMitarbeiter(t *testing.T) {
	a := newTestApp(t)
	w := roh(a, http.MethodPost, "/api/mitarbeiter/Weg/restore",
		`{"name":"Weg","entries":{"2026-09-01":{"frueh":true}}}`)
	if !strings.Contains(w.Body.String(), "nicht gefunden") {
		t.Fatalf("Wiederherstellen für Unbekannte: %s", w.Body.String())
	}
	if got := entered(t, a, "2026-09-01", "frueh"); len(got) != 0 {
		t.Fatalf("es wurde doch etwas angelegt: %v", got)
	}
}

func TestWiederherstellenUebergehtUnsinnigeEintraege(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	res := call(t, a, http.MethodPost, "/api/mitarbeiter/Anna/restore",
		`{"name":"Anna","entries":{"morgen":{"frueh":true},"2026-09-01":{"nacht":true},`+
			`"2026-09-02":{"frueh":true}}}`)
	if res["restored"] != 1.0 {
		t.Fatalf("nur ein Eintrag ist brauchbar: %v", res["restored"])
	}
	if got := entered(t, a, "2026-09-02", "frueh"); len(got) != 1 {
		t.Fatalf("der brauchbare Eintrag fehlt: %v", got)
	}
}

// ── Soll ──────────────────────────────────────────────────────────────────────

func TestSollBleibtImSinnvollenRahmen(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/soll", `{"frueh":-3,"normal":0,"spaet":500,"rufbereitschaft":1}`)

	data := call(t, a, http.MethodGet, "/api/data", "")
	soll, _ := data["soll"].(map[string]any)
	if soll["frueh"] != 0.0 {
		t.Errorf("negatives Soll: %v", soll["frueh"])
	}
	if soll["spaet"] != 99.0 {
		t.Errorf("Soll ohne Deckel: %v", soll["spaet"])
	}
	if soll["normal"] != 0.0 {
		t.Errorf("die 0 muss 0 bleiben: %v", soll["normal"])
	}
}

// ── Feiertage ─────────────────────────────────────────────────────────────────

func TestFeiertagsjahrBleibtImPlanbarenBereich(t *testing.T) {
	a := newTestApp(t)
	for _, jahr := range []string{"1", "1969", "2201", "9999", "-5", "zweitausend"} {
		if w := roh(a, http.MethodGet, "/api/holidays/"+jahr, ""); w.Code != http.StatusBadRequest {
			t.Errorf("Jahr %s: Status %d, erwartet 400", jahr, w.Code)
		}
	}
	for _, jahr := range []string{"1970", "2026", "2200"} {
		if w := roh(a, http.MethodGet, "/api/holidays/"+jahr, ""); w.Code != http.StatusOK {
			t.Errorf("Jahr %s: Status %d", jahr, w.Code)
		}
	}
}

func TestEigenerFeiertagBrauchtTagUndNamen(t *testing.T) {
	a := newTestApp(t)
	schlecht := []string{
		`{"name":"X","country":"DE"}`,
		`{"date":"","name":"X","country":"DE"}`,
		`{"date":"31.12.2026","name":"X","country":"DE"}`,
		`{"date":"2026-02-31","name":"X","country":"DE"}`,
		`{"date":"2026-08-14","name":"","country":"DE"}`,
		`{"date":"2026-08-14","name":"   ","country":"DE"}`,
	}
	for _, k := range schlecht {
		if w := roh(a, http.MethodPost, "/api/custom_holidays", k); !strings.Contains(w.Body.String(), "error") {
			t.Errorf("%s wurde angenommen: %s", k, w.Body.String())
		}
	}
	if liste := listeVon(t, a, "/api/custom_holidays"); len(liste) != 0 {
		t.Fatalf("es ist doch etwas angelegt worden: %v", liste)
	}

	call(t, a, http.MethodPost, "/api/custom_holidays", `{"date":"2026-08-14","name":"Ausflug","country":"DE"}`)
	if liste := listeVon(t, a, "/api/custom_holidays"); len(liste) != 1 {
		t.Fatalf("der brauchbare Feiertag fehlt: %v", liste)
	}
}

// ── ICS ───────────────────────────────────────────────────────────────────────

func icsImport(t *testing.T, a *Server, inhalt string) map[string]any {
	t.Helper()
	koerper, ctype := multipartBody(t, "cal.ics", inhalt)
	r := httptest.NewRequest(http.MethodPost, "/api/import_ics", strings.NewReader(koerper))
	r.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return jsonVon(t, w.Body.Bytes())
}

func icsTermin(datum, summary string) string {
	return "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nDTSTART:" + datum + "\r\nSUMMARY:" + summary +
		"\r\nEND:VEVENT\r\nEND:VCALENDAR"
}

func TestICSImportUebergehtWasErNichtEintragenKann(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	faelle := []struct{ name, inhalt string }{
		{"leere Datei", ""},
		{"kein Kalender", "Hallo Welt"},
		{"ohne DTSTART", "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nSUMMARY:Anna – Frühschicht\r\nEND:VEVENT\r\nEND:VCALENDAR"},
		{"unbekannte Schicht", icsTermin("20260401T060000", "Anna – Nachtdienst")},
		{"Datum gibt es nicht", icsTermin("20260231T060000", "Anna – Frühschicht")},
		{"Datum zu kurz", icsTermin("2026", "Anna – Frühschicht")},
		{"kein Trennstrich", icsTermin("20260401T060000", "Anna Frühschicht")},
	}
	for _, f := range faelle {
		res := icsImport(t, a, f.inhalt)
		if res["imported"] != 0.0 {
			t.Errorf("%s: %v importiert", f.name, res["imported"])
		}
	}
	data := call(t, a, http.MethodGet, "/api/data", "")
	if tage, _ := data["schichten"].(map[string]any); len(tage) != 0 {
		t.Fatalf("es ist doch etwas eingetragen worden: %#v", tage)
	}
}

func TestICSImportNenntUnbekannteNamen(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	res := icsImport(t, a, "BEGIN:VCALENDAR\r\n"+
		"BEGIN:VEVENT\r\nDTSTART:20260401T060000\r\nSUMMARY:Anna – Frühschicht\r\nEND:VEVENT\r\n"+
		"BEGIN:VEVENT\r\nDTSTART:20260401T140000\r\nSUMMARY:Niemand – Spätschicht\r\nEND:VEVENT\r\n"+
		"END:VCALENDAR")

	if res["imported"] != 1.0 {
		t.Fatalf("Anna hätte durchkommen müssen: %v", res)
	}
	unbekannt, _ := res["unbekannt"].([]any)
	if len(unbekannt) != 1 || unbekannt[0] != "Niemand" {
		t.Fatalf("der unbekannte Name wird nicht gemeldet: %#v", res["unbekannt"])
	}
	if got := entered(t, a, "2026-04-01", "spaet"); len(got) != 0 {
		t.Fatalf("der Unbekannte wurde eingetragen: %v", got)
	}
}

func TestICSImportVertraegtZeilenendenBeiderArt(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")

	mitCRLF := icsTermin("20260401T060000", "Anna – Frühschicht")
	if res := icsImport(t, a, mitCRLF); res["imported"] != 1.0 {
		t.Fatalf("CRLF: %v", res)
	}

	b := newTestApp(t)
	addEmployee(t, b, "Anna", "DE")
	if res := icsImport(t, b, strings.ReplaceAll(mitCRLF, "\r\n", "\n")); res["imported"] != 1.0 {
		t.Fatalf("nur LF: %v", res)
	}
}

func TestICSImportLegtNichtsDoppeltAn(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Anna", "DE")
	kalender := icsTermin("20260401T060000", "Anna – Frühschicht")

	if res := icsImport(t, a, kalender); res["imported"] != 1.0 {
		t.Fatalf("erster Durchgang: %v", res)
	}
	if res := icsImport(t, a, kalender); res["imported"] != 0.0 {
		t.Fatalf("zweiter Durchgang hat nochmal eingetragen: %v", res)
	}
	if got := entered(t, a, "2026-04-01", "frueh"); len(got) != 1 {
		t.Fatalf("Eintrag steht doppelt: %v", got)
	}
}

// ── Sicherung ─────────────────────────────────────────────────────────────────

func datenImport(t *testing.T, a *Server, inhalt string) map[string]any {
	t.Helper()
	koerper, ctype := multipartBody(t, "b.json", inhalt)
	r := httptest.NewRequest(http.MethodPost, "/api/import_data", strings.NewReader(koerper))
	r.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return jsonVon(t, w.Body.Bytes())
}

func TestSicherungWeistUnbrauchbaresAb(t *testing.T) {
	a := newTestApp(t)
	for _, inhalt := range []string{"", "kein json", `[1,2,3]`, `"nur ein Text"`} {
		res := datenImport(t, a, inhalt)
		if res["error"] == nil {
			t.Errorf("%q wurde angenommen: %#v", inhalt, res)
		}
	}
}

func TestSicherungSiebtNamenloseUndUnsinnigeTageAus(t *testing.T) {
	a := newTestApp(t)
	datenImport(t, a, `{"mitarbeiter":[{"name":"Anna","team":"DE"},{"name":"","team":"DE"},`+
		`{"name":"  ","team":"DE"},{"name":"Anna","team":"IN"}],`+
		`"schichten":{"morgen":{"frueh":["Anna"]},"2026-02-31":{"frueh":["Anna"]},`+
		`"2026-04-01":{"frueh":["Anna"]}},`+
		`"notizen":{"morgen":"x","2026-04-01":"gut"}}`)

	data := call(t, a, http.MethodGet, "/api/data", "")
	leute, _ := data["mitarbeiter"].([]any)
	if len(leute) != 1 {
		t.Fatalf("namenlose oder doppelte Mitarbeiter durchgelassen: %#v", leute)
	}
	tage, _ := data["schichten"].(map[string]any)
	if len(tage) != 1 {
		t.Fatalf("Unsinnstage durchgelassen: %#v", tage)
	}
	notizen, _ := data["notizen"].(map[string]any)
	if len(notizen) != 1 {
		t.Fatalf("Unsinnsnotizen durchgelassen: %#v", notizen)
	}
}

func TestSicherungUeberstehtDenRundlaufMitSonderzeichen(t *testing.T) {
	a := newTestApp(t)
	name := "Müller-Lüdenscheidt, Jörg \"der Ältere\""
	addEmployee(t, a, name, "DE")
	addShift(t, a, "2026-04-01", "frueh", name)
	call(t, a, http.MethodPost, "/api/notiz", `{"date":"2026-04-01","text":"Übergabe – 14 Uhr ☕"}`)

	r := httptest.NewRequest(http.MethodGet, "/api/export_data", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	sicherung := w.Body.String()

	b := newTestApp(t)
	datenImport(t, b, sicherung)

	if got := entered(t, b, "2026-04-01", "frueh"); len(got) != 1 || got[0] != name {
		t.Fatalf("Name kam nicht durch: %#v", got)
	}
	data := call(t, b, http.MethodGet, "/api/data", "")
	notizen, _ := data["notizen"].(map[string]any)
	if notizen["2026-04-01"] != "Übergabe – 14 Uhr ☕" {
		t.Fatalf("Notiz kam nicht durch: %#v", notizen)
	}
}

// ── Gelöschte Mitarbeiter ─────────────────────────────────────────────────────

func TestKWUebertragungUebergehtGeloeschteMitarbeiter(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Weg", "DE")
	addEmployee(t, a, "Bleibt", "DE")
	call(t, a, http.MethodPost, "/api/ruf_kw",
		`{"ruf_kw":{"2026-W10":["Weg"],"2026-W11":["Bleibt"]}}`)

	roh(a, http.MethodDelete, "/api/mitarbeiter/Weg", "")
	res := call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026}`)

	// Nur die Woche von "Bleibt" wird eingetragen.
	if res["applied"] != 7.0 {
		t.Fatalf("eingetragen: %v", res["applied"])
	}
	unbekannt, _ := res["unbekannt"].([]any)
	if len(unbekannt) != 1 || unbekannt[0] != "Weg" {
		t.Fatalf("der fehlende Name wird nicht gemeldet: %#v", res["unbekannt"])
	}
	if got := entered(t, a, "2026-03-02", "rufbereitschaft"); len(got) != 0 {
		t.Fatalf("Woche des Gelöschten eingetragen: %v", got)
	}
}

func TestAutoplanUebergehtGeloeschteMitarbeiter(t *testing.T) {
	a := newTestApp(t)
	addEmployee(t, a, "Weg", "DE")
	addEmployee(t, a, "Bleibt", "DE")
	call(t, a, http.MethodPost, "/api/templates",
		`{"name":"t","template":{"Weg":{"0":"frueh"},"Bleibt":{"1":"spaet"}}}`)

	roh(a, http.MethodDelete, "/api/mitarbeiter/Weg", "")
	res := call(t, a, http.MethodPost, "/api/autoplan", `{"year":2026,"month":6,"template":"t"}`)

	// Juni 2026 hat fünf Dienstage - nur die von "Bleibt" werden geplant.
	if res["planned"] != 5.0 {
		t.Fatalf("geplant: %v", res["planned"])
	}
	unbekannt, _ := res["unbekannt"].([]any)
	if len(unbekannt) != 1 || unbekannt[0] != "Weg" {
		t.Fatalf("der fehlende Name wird nicht gemeldet: %#v", res["unbekannt"])
	}
	if got := entered(t, a, "2026-06-01", "frueh"); len(got) != 0 {
		t.Fatalf("Montag des Gelöschten geplant: %v", got)
	}
}
