package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Diese Datei deckt ab, was handlers_test.go offen laesst: die Handler hinter
// den Bedienschritten der Oberflaeche (Tag leeren, kopieren, einfuegen, Soll,
// Notiz, Rueckgaengig), die Antwort auf unsinnige Eingaben und das Verhalten
// unter gleichzeitigen Zugriffen.

// roh schickt eine Anfrage durch den Router und gibt den Recorder zurueck -
// anders als call() ohne Anspruch darauf, dass die Antwort 200 und JSON ist.
func roh(a *App, method, path, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

// mustJSON baut einen JSON-Koerper aus Go-Werten - fuer Eingaben, die sich
// nicht gefahrlos als Zeichenkette zusammenkleben lassen.
func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// tagInhalt liefert die Namen einer Schicht an einem Tag.
func tagInhalt(t *testing.T, a *App, datum, schicht string) []string {
	t.Helper()
	return entered(t, a, datum, schicht)
}

// ── Tag leeren, kopieren, einfuegen ───────────────────────────────────────────

func TestPasteReplaceLeertDenTagVollstaendig(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-09-01"],"schicht":"frueh","name":"Bauer","action":"add"}`)
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-09-01"],"schicht":"rufbereitschaft","name":"Bauer","action":"add"}`)

	// Genau das schickt "Tag leeren" aus dem Tagesmenue.
	call(t, a, http.MethodPost, "/api/paste",
		`{"slot":{"frueh":[],"normal":[],"spaet":[],"rufbereitschaft":[]},"dates":["2026-09-01"],"mode":"replace"}`)

	for _, schicht := range []string{"frueh", "rufbereitschaft"} {
		if got := tagInhalt(t, a, "2026-09-01", schicht); len(got) != 0 {
			t.Fatalf("%s nach dem Leeren: %v", schicht, got)
		}
	}
}

func TestPasteReplaceUeberschreibtStattZuErgaenzen(t *testing.T) {
	a := newTestApp(t)
	for _, n := range []string{"Bauer", "Wolf"} {
		call(t, a, http.MethodPost, "/api/mitarbeiter", fmt.Sprintf(`{"name":%q,"team":"DE"}`, n))
	}
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-09-01"],"schicht":"frueh","name":"Bauer","action":"add"}`)

	call(t, a, http.MethodPost, "/api/paste",
		`{"slot":{"frueh":["Wolf"]},"dates":["2026-09-01"],"mode":"replace"}`)

	got := tagInhalt(t, a, "2026-09-01", "frueh")
	if len(got) != 1 || got[0] != "Wolf" {
		t.Fatalf("replace haette Bauer ersetzen muessen, steht: %v", got)
	}
}

func TestPasteMergeLaesstBestehendesStehen(t *testing.T) {
	a := newTestApp(t)
	for _, n := range []string{"Bauer", "Wolf"} {
		call(t, a, http.MethodPost, "/api/mitarbeiter", fmt.Sprintf(`{"name":%q,"team":"DE"}`, n))
	}
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-09-01"],"schicht":"frueh","name":"Bauer","action":"add"}`)

	// Ohne "mode" faellt der Handler auf merge zurueck.
	call(t, a, http.MethodPost, "/api/paste",
		`{"slot":{"frueh":["Wolf"]},"dates":["2026-09-01"]}`)

	got := tagInhalt(t, a, "2026-09-01", "frueh")
	if len(got) != 2 {
		t.Fatalf("merge haette beide behalten muessen, steht: %v", got)
	}
}

func TestPasteAufMehrereTageUndAufKeinenTag(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)

	tage := []string{"2026-09-01", "2026-09-02", "2026-09-03"}
	call(t, a, http.MethodPost, "/api/paste",
		`{"slot":{"frueh":["Bauer"]},"dates":["2026-09-01","2026-09-02","2026-09-03"],"mode":"replace"}`)
	for _, d := range tage {
		if got := tagInhalt(t, a, d, "frueh"); len(got) != 1 {
			t.Fatalf("%s: %v", d, got)
		}
	}

	// Eine leere Tagesliste ist kein Fehler, sie tut nur nichts.
	call(t, a, http.MethodPost, "/api/paste", `{"slot":{"frueh":["Bauer"]},"dates":[],"mode":"replace"}`)
	if got := tagInhalt(t, a, "2026-09-01", "frueh"); len(got) != 1 {
		t.Fatalf("leere Liste haette nichts aendern duerfen: %v", got)
	}
}

// ── Soll ──────────────────────────────────────────────────────────────────────

func TestSollWirdGespeichertUndNullBleibtNull(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/soll",
		`{"frueh":2,"normal":0,"spaet":1,"rufbereitschaft":1}`)

	data := call(t, a, http.MethodGet, "/api/data", "")
	soll, ok := data["soll"].(map[string]interface{})
	if !ok {
		t.Fatalf("kein Soll in /api/data: %v", data["soll"])
	}
	if soll["frueh"] != 2.0 {
		t.Fatalf("frueh: %v", soll["frueh"])
	}
	// Der Normaldienst steht ueblicherweise auf 0. Kaeme hier 1 zurueck,
	// zaehlte die Oberflaeche jeden Tag als unterbesetzt.
	if soll["normal"] != 0.0 {
		t.Fatalf("normal haette 0 bleiben muessen, ist: %v", soll["normal"])
	}
}

// ── Notiz ─────────────────────────────────────────────────────────────────────

func TestNotizSchreibenAendernLeeren(t *testing.T) {
	a := newTestApp(t)
	notiz := func() string {
		data := call(t, a, http.MethodGet, "/api/data", "")
		n, _ := data["notizen"].(map[string]interface{})
		s, _ := n["2026-09-01"].(string)
		return s
	}

	call(t, a, http.MethodPost, "/api/notiz", `{"date":"2026-09-01","text":"Übergabe 14 Uhr"}`)
	if notiz() != "Übergabe 14 Uhr" {
		t.Fatalf("Notiz nicht gespeichert: %q", notiz())
	}

	call(t, a, http.MethodPost, "/api/notiz", `{"date":"2026-09-01","text":"verschoben"}`)
	if notiz() != "verschoben" {
		t.Fatalf("Notiz nicht geaendert: %q", notiz())
	}

	call(t, a, http.MethodPost, "/api/notiz", `{"date":"2026-09-01","text":""}`)
	if notiz() != "" {
		t.Fatalf("leere Notiz haette den Eintrag loeschen muessen: %q", notiz())
	}
}

// ── Rueckgaengig / Wiederholen ────────────────────────────────────────────────

func TestSnapshotSetztDenGanzenPlanZurueck(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-09-01","2026-09-02"],"schicht":"frueh","name":"Bauer","action":"add"}`)

	// So sieht der Undo-Sprung aus: die Oberflaeche schickt den alten Stand.
	call(t, a, http.MethodPost, "/api/snapshot",
		`{"schichten":{"2026-09-01":{"frueh":["Bauer"],"normal":[],"spaet":[],"rufbereitschaft":[]}}}`)

	if got := tagInhalt(t, a, "2026-09-01", "frueh"); len(got) != 1 {
		t.Fatalf("1.9. haette bleiben muessen: %v", got)
	}
	if got := tagInhalt(t, a, "2026-09-02", "frueh"); len(got) != 0 {
		t.Fatalf("2.9. haette verschwinden muessen: %v", got)
	}
}

func TestSnapshotOhneDatenAendertNichts(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-09-01"],"schicht":"frueh","name":"Bauer","action":"add"}`)

	// null statt einer Karte: der Handler bestaetigt, ruehrt aber nichts an -
	// sonst loeschte ein leerer Undo-Stapel den ganzen Plan.
	call(t, a, http.MethodPost, "/api/snapshot", `{"schichten":null}`)

	if got := tagInhalt(t, a, "2026-09-01", "frueh"); len(got) != 1 {
		t.Fatalf("Plan haette unberuehrt bleiben muessen: %v", got)
	}
}

// ── Unsinnige Eingaben ────────────────────────────────────────────────────────

func TestKaputterJSONKoerperGibt400(t *testing.T) {
	a := newTestApp(t)
	faelle := []struct{ pfad, koerper string }{
		{"/api/schicht", `{"dates":`},
		{"/api/paste", `nicht mal JSON`},
		{"/api/soll", `[1,2,3]`},
		{"/api/notiz", `{"date":42}`},
		{"/api/snapshot", `{"schichten":"Text statt Karte"}`},
		{"/api/mitarbeiter", `{`},
	}
	for _, f := range faelle {
		w := roh(a, http.MethodPost, f.pfad, f.koerper)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s mit %q: Status %d, erwartet 400", f.pfad, f.koerper, w.Code)
		}
	}
}

func TestUnbekannteWegeUndFalscheMethodenGeben404(t *testing.T) {
	a := newTestApp(t)
	faelle := []struct{ methode, pfad string }{
		{http.MethodGet, "/api/gibtsnicht"},
		{http.MethodGet, "/api/schicht"}, // nur POST
		{http.MethodGet, "/api/paste"},   // nur POST
		{http.MethodDelete, "/api/soll"}, // nur POST
		{http.MethodPost, "/api/mitarbeiter/Bauer/gibtsnicht"},
	}
	for _, f := range faelle {
		w := roh(a, f.methode, f.pfad, "")
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s: Status %d, erwartet 404", f.methode, f.pfad, w.Code)
		}
	}
}

func TestLesendeWegeAntwortenAufJedeMethode(t *testing.T) {
	a := newTestApp(t)
	// Festgehalten, wie es ist: /api/data, /api/history und die uebrigen
	// Lesewege pruefen die Methode nicht, ein POST liest also genauso.
	// Harmlos, weil die Handler nichts schreiben - aber es ist Absicht der
	// Routing-Tabelle, keine Nachlaessigkeit dieses Tests.
	for _, pfad := range []string{"/api/data", "/api/history", "/api/datadir", "/api/holiday_coverage"} {
		for _, methode := range []string{http.MethodGet, http.MethodPost} {
			w := roh(a, methode, pfad, "")
			if w.Code != http.StatusOK {
				t.Errorf("%s %s: Status %d", methode, pfad, w.Code)
			}
		}
	}
}

func TestWurzelLiefertDieOberflaeche(t *testing.T) {
	a := newTestApp(t)
	for _, pfad := range []string{"/", "/index.html"} {
		w := roh(a, http.MethodGet, pfad, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: Status %d", pfad, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("%s: Content-Type %q", pfad, ct)
		}
		// Der Regelkern muss mitgeliefert werden - ohne ihn startet die
		// Oberflaeche nicht, und tests/regelkern.test.mjs findet ihn nicht.
		if !strings.Contains(w.Body.String(), `<script id="regelkern">`) {
			t.Fatalf("%s: der Regelkern fehlt in der ausgelieferten Seite", pfad)
		}
	}
}

func TestMitarbeiterMitLeeremNamenWirdAbgelehnt(t *testing.T) {
	a := newTestApp(t)
	for _, koerper := range []string{`{"name":"","team":"DE"}`, `{"name":"   ","team":"DE"}`} {
		w := roh(a, http.MethodPost, "/api/mitarbeiter", koerper)
		if w.Code == http.StatusOK && !strings.Contains(w.Body.String(), "error") {
			t.Errorf("%s wurde angenommen: %s", koerper, w.Body.String())
		}
	}
}

func TestNamenMitSonderzeichenUndLaengeUeberlebenDenRundlauf(t *testing.T) {
	a := newTestApp(t)
	namen := []string{
		"Müller-Lüdenscheidt, Jörg",
		"अनिता नायर",
		"O'Brien, Seán",
		strings.Repeat("L", 200),
		`Anführungs"zeichen`,
	}
	for _, n := range namen {
		w := roh(a, http.MethodPost, "/api/mitarbeiter", mustJSON(map[string]string{"name": n, "team": "DE"}))
		if w.Code != http.StatusOK {
			t.Fatalf("%q: Status %d (%s)", n, w.Code, w.Body.String())
		}
		call(t, a, http.MethodPost, "/api/schicht",
			mustJSON(map[string]interface{}{
				"dates": []string{"2026-09-01"}, "schicht": "frueh", "name": n, "action": "add",
			}))
		got := tagInhalt(t, a, "2026-09-01", "frueh")
		if len(got) == 0 || got[len(got)-1] != n {
			t.Fatalf("%q kam nicht zurueck: %v", n, got)
		}
	}
}

// ── Gleichzeitige Zugriffe ────────────────────────────────────────────────────

func TestGleichzeitigeEintraegeAufDenselbenTag(t *testing.T) {
	a := newTestApp(t)
	const anzahl = 24
	for i := 0; i < anzahl; i++ {
		call(t, a, http.MethodPost, "/api/mitarbeiter",
			fmt.Sprintf(`{"name":"P%02d","team":"DE"}`, i))
	}

	// Alle tragen sich gleichzeitig in dieselbe Schicht desselben Tages ein.
	// Der Mutex in ServeHTTP muss das serialisieren; sonst gehen Eintraege
	// verloren oder die Transaktionen kollidieren.
	var wg sync.WaitGroup
	fehler := make(chan string, anzahl)
	for i := 0; i < anzahl; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := roh(a, http.MethodPost, "/api/schicht", fmt.Sprintf(
				`{"dates":["2026-09-01"],"schicht":"rufbereitschaft","name":"P%02d","action":"add"}`, i))
			if w.Code != http.StatusOK {
				fehler <- fmt.Sprintf("P%02d: Status %d (%s)", i, w.Code, w.Body.String())
			}
		}(i)
	}
	wg.Wait()
	close(fehler)
	for f := range fehler {
		t.Error(f)
	}

	got := tagInhalt(t, a, "2026-09-01", "rufbereitschaft")
	if len(got) != anzahl {
		t.Fatalf("%d von %d Eintraegen ueberlebt: %v", len(got), anzahl, got)
	}
	gesehen := map[string]bool{}
	for _, n := range got {
		if gesehen[n] {
			t.Fatalf("%s steht doppelt im Tag: %v", n, got)
		}
		gesehen[n] = true
	}
}

func TestLesenUndSchreibenDurcheinander(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)

	var wg sync.WaitGroup
	schlecht := make(chan string, 60)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) { // schreiben
			defer wg.Done()
			datum := fmt.Sprintf("2026-09-%02d", i%28+1)
			w := roh(a, http.MethodPost, "/api/schicht", fmt.Sprintf(
				`{"dates":[%q],"schicht":"frueh","name":"Bauer","action":"add"}`, datum))
			if w.Code != http.StatusOK {
				schlecht <- fmt.Sprintf("Schreiben %s: %d", datum, w.Code)
			}
		}(i)
		wg.Add(1)
		go func() { // lesen
			defer wg.Done()
			w := roh(a, http.MethodGet, "/api/data", "")
			if w.Code != http.StatusOK {
				schlecht <- fmt.Sprintf("Lesen: %d", w.Code)
			}
			if !strings.HasPrefix(strings.TrimSpace(w.Body.String()), "{") {
				schlecht <- "Lesen: keine JSON-Karte zurueck"
			}
		}()
	}
	wg.Wait()
	close(schlecht)
	for s := range schlecht {
		t.Error(s)
	}
}

func TestGleichzeitigesLeerenUndEintragenLaesstKeinenHalbenTagZurueck(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			roh(a, http.MethodPost, "/api/schicht",
				`{"dates":["2026-09-01"],"schicht":"frueh","name":"Bauer","action":"add"}`)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			roh(a, http.MethodPost, "/api/paste",
				`{"slot":{"frueh":[],"normal":[],"spaet":[],"rufbereitschaft":[]},"dates":["2026-09-01"],"mode":"replace"}`)
		}()
	}
	wg.Wait()

	// Wie das Rennen ausgeht, ist offen - aber der Tag muss danach entweder
	// leer sein oder genau einen Eintrag haben, nie zwanzig Duplikate.
	got := tagInhalt(t, a, "2026-09-01", "frueh")
	if len(got) > 1 {
		t.Fatalf("Tag steht mehrfach besetzt da: %v", got)
	}
}
