package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Die schmalen Lese- und Schreibwege, die handlers_test.go auslaesst:
// Mitarbeiterliste, Farbe, Praeferenzen, Templates, eigene Feiertage und der
// KW-Plan. Kurz, aber sie sind der Unterschied zwischen "laeuft bestimmt" und
// "laeuft nachweislich".

// listeVon dekodiert eine JSON-Liste aus einer Antwort.
func listeVon(t *testing.T, a *App, pfad string) []interface{} {
	t.Helper()
	w := roh(a, http.MethodGet, pfad, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: Status %d (%s)", pfad, w.Code, w.Body.String())
	}
	var out []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("GET %s: kein JSON-Array: %s", pfad, w.Body.String())
	}
	return out
}

func TestMitarbeiterlisteWaechstUndSchrumpft(t *testing.T) {
	a := newTestApp(t)
	if got := listeVon(t, a, "/api/mitarbeiter"); len(got) != 0 {
		t.Fatalf("frische Ablage ist nicht leer: %v", got)
	}
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Nair","team":"IN"}`)
	if got := listeVon(t, a, "/api/mitarbeiter"); len(got) != 2 {
		t.Fatalf("erwartet 2, bekommen %d", len(got))
	}
	roh(a, http.MethodDelete, "/api/mitarbeiter/Bauer", "")
	if got := listeVon(t, a, "/api/mitarbeiter"); len(got) != 1 {
		t.Fatalf("nach dem Loeschen erwartet 1, bekommen %d", len(got))
	}
}

func TestFarbeUndPraeferenzenBleibenAmMitarbeiter(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE","color":"#111111"}`)

	call(t, a, http.MethodPost, "/api/mitarbeiter/Bauer/color", `{"color":"#4a9eff"}`)
	call(t, a, http.MethodPost, "/api/mitarbeiter/Bauer/prefs", `{"prefs":{"Mo":"frueh","Fr":"spaet"}}`)

	data := call(t, a, http.MethodGet, "/api/data", "")
	leute, _ := data["mitarbeiter"].([]interface{})
	if len(leute) != 1 {
		t.Fatalf("mitarbeiter: %v", data["mitarbeiter"])
	}
	m, _ := leute[0].(map[string]interface{})
	if m["color"] != "#4a9eff" {
		t.Fatalf("Farbe: %v", m["color"])
	}
	prefs, _ := m["prefs"].(map[string]interface{})
	if prefs["Mo"] != "frueh" || prefs["Fr"] != "spaet" {
		t.Fatalf("Praeferenzen: %v", prefs)
	}
}

func TestFarbeUndPraeferenzenWeisenKaputtenKoerperAb(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)
	for _, pfad := range []string{"/api/mitarbeiter/Bauer/color", "/api/mitarbeiter/Bauer/prefs"} {
		if w := roh(a, http.MethodPost, pfad, `{`); w.Code != http.StatusBadRequest {
			t.Errorf("%s: Status %d, erwartet 400", pfad, w.Code)
		}
	}
}

func TestTemplateAnlegenLesenLoeschen(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)

	call(t, a, http.MethodPost, "/api/templates",
		`{"name":"woche","template":{"Bauer":{"1":"frueh","5":"spaet"}}}`)

	w := roh(a, http.MethodGet, "/api/templates", "")
	if !strings.Contains(w.Body.String(), "woche") {
		t.Fatalf("Template nicht in der Liste: %s", w.Body.String())
	}

	// Ein Template ohne Namen ist keins.
	if w := roh(a, http.MethodPost, "/api/templates", `{"template":{}}`); !strings.Contains(w.Body.String(), "error") {
		t.Fatalf("namenloses Template wurde angenommen: %s", w.Body.String())
	}

	roh(a, http.MethodDelete, "/api/templates/woche", "")
	if w := roh(a, http.MethodGet, "/api/templates", ""); strings.Contains(w.Body.String(), "woche") {
		t.Fatalf("Template ueberlebt das Loeschen: %s", w.Body.String())
	}
}

func TestEigeneFeiertageAnlegenLesenLoeschen(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/custom_holidays",
		`{"date":"2026-08-14","name":"Betriebsausflug","country":"DE"}`)

	liste := listeVon(t, a, "/api/custom_holidays")
	if len(liste) != 1 {
		t.Fatalf("erwartet 1 eigenen Feiertag, bekommen %d", len(liste))
	}

	// Der Schluessel ist "Datum|Name" - ein Datum allein reicht nicht, weil an
	// einem Tag mehrere eigene Feiertage stehen koennen.
	if w := roh(a, http.MethodDelete, "/api/custom_holidays/2026-08-14", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("halber Schluessel: Status %d, erwartet 400", w.Code)
	}
	if liste := listeVon(t, a, "/api/custom_holidays"); len(liste) != 1 {
		t.Fatalf("der abgelehnte Loeschversuch hat trotzdem etwas entfernt: %v", liste)
	}

	pfad := "/api/custom_holidays/" + url.PathEscape("2026-08-14|Betriebsausflug")
	if w := roh(a, http.MethodDelete, pfad, ""); w.Code != http.StatusOK {
		t.Fatalf("Loeschen: Status %d (%s)", w.Code, w.Body.String())
	}
	if liste := listeVon(t, a, "/api/custom_holidays"); len(liste) != 0 {
		t.Fatalf("Feiertag ueberlebt das Loeschen: %v", liste)
	}
}

func TestRufKWPlanKommtLeerUndGefuelltZurueck(t *testing.T) {
	a := newTestApp(t)
	w := roh(a, http.MethodGet, "/api/ruf_kw", "")
	if w.Code != http.StatusOK {
		t.Fatalf("leerer Plan: Status %d", w.Code)
	}

	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Nair","team":"IN"}`)
	call(t, a, http.MethodPost, "/api/ruf_kw", `{"2026-KW36":["Nair"]}`)

	w = roh(a, http.MethodGet, "/api/ruf_kw", "")
	if !strings.Contains(w.Body.String(), "Nair") {
		t.Fatalf("gespeicherte Woche fehlt: %s", w.Body.String())
	}
}

func TestVerlaufHaeltDieLetztenAenderungenFest(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Bauer","team":"DE"}`)
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-09-01"],"schicht":"frueh","name":"Bauer","action":"add"}`)

	// Ohne brauchbares limit greift der Vorgabewert - beide Wege muessen
	// eine Liste liefern, keinen Fehler.
	for _, abfrage := range []string{"", "?limit=1", "?limit=0", "?limit=99999", "?limit=abc"} {
		liste := listeVon(t, a, "/api/history"+abfrage)
		if len(liste) == 0 {
			t.Fatalf("history%s ist leer", abfrage)
		}
	}
	if liste := listeVon(t, a, "/api/history?limit=1"); len(liste) != 1 {
		t.Fatalf("limit=1 lieferte %d Eintraege", len(liste))
	}
}

func TestDatenordnerWirdGemeldet(t *testing.T) {
	a := newTestApp(t)
	data := call(t, a, http.MethodGet, "/api/datadir", "")
	if data["folder"] == "" || data["file"] == "" {
		t.Fatalf("Ordner oder Datei fehlt: %v", data)
	}
}

func TestOrdnerwechselWeistUnbrauchbareAngabenAb(t *testing.T) {
	a := newTestApp(t)
	vorher := call(t, a, http.MethodGet, "/api/datadir", "")["folder"]

	faelle := []struct{ koerper, meldung string }{
		{`{"folder":""}`, "Kein Ordner"},
		{`{}`, "Kein Ordner"},
		{`{"folder":"C:\\gibt-es-nicht\\wirklich-nicht"}`, "Ordner nicht gefunden"},
		{`{"folder":"/gibt-es-nicht/wirklich-nicht"}`, "Ordner nicht gefunden"},
	}
	for _, f := range faelle {
		w := roh(a, http.MethodPost, "/api/set_datadir", f.koerper)
		if !strings.Contains(w.Body.String(), f.meldung) {
			t.Errorf("%s: erwartet %q, bekommen %s", f.koerper, f.meldung, w.Body.String())
		}
	}
	if w := roh(a, http.MethodPost, "/api/set_datadir", `{`); w.Code != http.StatusBadRequest {
		t.Errorf("kaputter Koerper: Status %d, erwartet 400", w.Code)
	}

	// Nach lauter abgelehnten Versuchen muss der alte Ordner stehen.
	if nachher := call(t, a, http.MethodGet, "/api/datadir", "")["folder"]; nachher != vorher {
		t.Fatalf("Ordner gewechselt: %v -> %v", vorher, nachher)
	}
}

// merkeKonfigWoanders lenkt die Konfigurationsdatei in ein Wegwerfverzeichnis
// um, damit der Ordnerwechsel nicht ins echte Benutzerprofil schreibt.
func merkeKonfigWoanders(t *testing.T) string {
	t.Helper()
	pfad := filepath.Join(t.TempDir(), "config.json")
	vorher := configPath
	configPath = func() string { return pfad }
	t.Cleanup(func() { configPath = vorher })
	return pfad
}

func TestOrdnerwechselSchaltetUmUndMerktSichDasZiel(t *testing.T) {
	a := newTestApp(t)
	konfig := merkeKonfigWoanders(t)
	alt := call(t, a, http.MethodGet, "/api/datadir", "")["folder"]
	neu := t.TempDir()

	antwort := call(t, a, http.MethodPost, "/api/set_datadir", mustJSON(map[string]string{"folder": neu}))
	if antwort["ok"] != true {
		t.Fatalf("Wechsel abgelehnt: %v", antwort)
	}
	if antwort["folder"] != neu {
		t.Fatalf("gemeldeter Ordner: %v, erwartet %v", antwort["folder"], neu)
	}
	if jetzt := call(t, a, http.MethodGet, "/api/datadir", "")["folder"]; jetzt != neu {
		t.Fatalf("Ordner nicht umgeschaltet: %v", jetzt)
	}
	if alt == neu {
		t.Fatal("der Test hat gar nichts gewechselt")
	}

	// Die Wahl muss den naechsten Start ueberleben.
	roh, err := os.ReadFile(konfig)
	if err != nil {
		t.Fatalf("Konfiguration nicht geschrieben: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(roh, &cfg); err != nil {
		t.Fatalf("Konfiguration ist kein JSON: %s", roh)
	}
	if cfg.DataFolder != neu {
		t.Fatalf("gemerkter Ordner: %q, erwartet %q", cfg.DataFolder, neu)
	}

	// Und der neue Ordner traegt danach eigene Daten, nicht die alten.
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Neu","team":"DE"}`)
	if liste := listeVon(t, a, "/api/mitarbeiter"); len(liste) != 1 {
		t.Fatalf("frischer Ordner ist nicht frisch: %v", liste)
	}
}

func TestKaputterZielordnerLaesstDenAltenStehen(t *testing.T) {
	a := newTestApp(t)
	merkeKonfigWoanders(t)
	alt := call(t, a, http.MethodGet, "/api/datadir", "")["folder"]

	// Eine Datei ist kein Ordner.
	datei := filepath.Join(t.TempDir(), "keine-ablage.txt")
	if err := os.WriteFile(datei, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := roh(a, http.MethodPost, "/api/set_datadir", mustJSON(map[string]string{"folder": datei}))
	if !strings.Contains(w.Body.String(), "Ordner nicht gefunden") {
		t.Fatalf("Datei als Ordner angenommen: %s", w.Body.String())
	}
	if jetzt := call(t, a, http.MethodGet, "/api/datadir", "")["folder"]; jetzt != alt {
		t.Fatalf("Ordner gewechselt: %v -> %v", alt, jetzt)
	}
}
