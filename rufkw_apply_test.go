package main

import (
	"fmt"
	"net/http"
	"sort"
	"testing"
)

// Der Weg vom KW-Plan in den Kalender. Gemeldet war, dass er "nicht
// zuverlässig" überträgt - im Frontend lag es am nicht gespeicherten Plan und
// an der Monatsvorbelegung, hier an den Wochen über der Jahresgrenze.

// rufTage liefert alle Tage, an denen jemand als Rufbereitschaft steht.
func rufTage(t *testing.T, a *App, name string) []string {
	t.Helper()
	data := call(t, a, http.MethodGet, "/api/data", "")
	tage, _ := data["schichten"].(map[string]interface{})
	var raus []string
	for datum, roh := range tage {
		tag, _ := roh.(map[string]interface{})
		liste, _ := tag["rufbereitschaft"].([]interface{})
		for _, n := range liste {
			if s, _ := n.(string); s == name {
				raus = append(raus, datum)
			}
		}
	}
	sort.Strings(raus)
	return raus
}

func planMitWoche(t *testing.T, a *App, kwKey, name string) {
	t.Helper()
	call(t, a, http.MethodPost, "/api/mitarbeiter", fmt.Sprintf(`{"name":%q,"team":"DE"}`, name))
	call(t, a, http.MethodPost, "/api/ruf_kw",
		fmt.Sprintf(`{"ruf_kw":{%q:[%q]}}`, kwKey, name))
}

func TestUebertragungFuelltDieGanzeWoche(t *testing.T) {
	a := newTestApp(t)
	planMitWoche(t, a, "2026-W10", "Nair")

	call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026}`)

	// KW 10 im Jahr 2026: Montag 2.3. bis Sonntag 8.3.
	got := rufTage(t, a, "Nair")
	want := []string{"2026-03-02", "2026-03-03", "2026-03-04",
		"2026-03-05", "2026-03-06", "2026-03-07", "2026-03-08"}
	if len(got) != len(want) {
		t.Fatalf("übertragen: %v, erwartet %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("übertragen: %v, erwartet %v", got, want)
		}
	}
}

func TestUebertragungReichtUeberDieJahresgrenze(t *testing.T) {
	a := newTestApp(t)
	// Die erste Kalenderwoche 2025 beginnt am Montag, dem 30.12.2024.
	planMitWoche(t, a, "2025-W01", "Nair")

	call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2025}`)

	got := rufTage(t, a, "Nair")
	want := []string{"2024-12-30", "2024-12-31", "2025-01-01",
		"2025-01-02", "2025-01-03", "2025-01-04", "2025-01-05"}
	if len(got) != len(want) {
		t.Fatalf("übertragen: %v\nerwartet:   %v\nDie Tage aus dem Dezember davor gehören zur KW 01 und fehlen sonst für immer:"+
			" beim Übertragen von 2024 zählen sie nicht mit, weil ihr Wochenschlüssel 2025-W01 ist.", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("übertragen: %v, erwartet %v", got, want)
		}
	}
}

func TestUebertragungFuerEinenMonatBleibtInDiesemMonat(t *testing.T) {
	a := newTestApp(t)
	planMitWoche(t, a, "2026-W10", "Nair")

	// KW 10 liegt ganz im März - ein Übertrag für den April darf nichts tun.
	call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026,"month":4}`)
	if got := rufTage(t, a, "Nair"); len(got) != 0 {
		t.Fatalf("April hat fremde Tage eingetragen: %v", got)
	}

	call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026,"month":3}`)
	if got := rufTage(t, a, "Nair"); len(got) != 7 {
		t.Fatalf("März: %v", got)
	}
}

func TestZweiteUebertragungLegtNichtsDoppeltAn(t *testing.T) {
	a := newTestApp(t)
	planMitWoche(t, a, "2026-W10", "Nair")

	erste := call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026}`)
	zweite := call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026}`)

	if erste["applied"] != 7.0 {
		t.Fatalf("erster Durchgang: %v", erste["applied"])
	}
	if zweite["applied"] != 0.0 {
		t.Fatalf("zweiter Durchgang hat nochmal eingetragen: %v", zweite["applied"])
	}
	if got := rufTage(t, a, "Nair"); len(got) != 7 {
		t.Fatalf("nach zwei Durchgängen: %v", got)
	}
}

func TestUebertragungNimmtMehrerePersonenProWoche(t *testing.T) {
	a := newTestApp(t)
	for _, n := range []string{"Nair", "Bauer"} {
		call(t, a, http.MethodPost, "/api/mitarbeiter", fmt.Sprintf(`{"name":%q,"team":"DE"}`, n))
	}
	call(t, a, http.MethodPost, "/api/ruf_kw", `{"ruf_kw":{"2026-W10":["Nair","Bauer"]}}`)

	call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026}`)

	if got := rufTage(t, a, "Nair"); len(got) != 7 {
		t.Fatalf("Nair: %v", got)
	}
	if got := rufTage(t, a, "Bauer"); len(got) != 7 {
		t.Fatalf("Bauer: %v", got)
	}
}

func TestUebertragungVertraegtEinenEinzelnenNamen(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Nair","team":"DE"}`)
	// Aeltere Stände haben je Woche einen Namen statt einer Liste gespeichert.
	call(t, a, http.MethodPost, "/api/ruf_kw", `{"ruf_kw":{"2026-W10":"Nair"}}`)

	call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026}`)

	if got := rufTage(t, a, "Nair"); len(got) != 7 {
		t.Fatalf("Einzelname: %v", got)
	}
}
