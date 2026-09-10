// Der Wochenplan der Rufbereitschaft und sein Weg in den Kalender.
package httpapi

import (
	"net/http"
	"schichtplaner/internal/domain"
	"schichtplaner/internal/store"
)

// ── /api/ruf_kw ──────────────────────────────────────────────────────────────

func (srv *Server) handleGetRufKW(w http.ResponseWriter, r *http.Request) {
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	plan, err := s.RufKW(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, plan)
}

func (srv *Server) handleSaveRufKW(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// Die Oberfläche schickt {"ruf_kw": {"2026-W12": [...]}} - abgelegt wird die
	// innere Karte, sonst liegen die Wochenschlüssel eine Ebene zu tief und
	// Übertragen wie Neuladen finden sie nicht mehr.
	plan := domain.UnwrapRufKW(body)
	if srv.write(w, func(s *store.Store) error { return s.SaveRufKW(r.Context(), plan) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (srv *Server) handleApplyRufKW(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	plan, err := s.RufKW(r.Context())
	if err != nil {
		fail(w, err)
		return
	}

	namen, err := srv.bekannteNamen(r.Context(), s)
	if err != nil {
		fail(w, err)
		return
	}
	// Im Wochenplan kann jemand stehen, der inzwischen geloescht wurde. Seine
	// Wochen werden uebergangen statt als namenlose Eintraege angelegt.
	alle := domain.RufKWChanges(plan, body.Year, body.Month)
	changes := alle[:0]
	fehlend := map[string]bool{}
	for _, c := range alle {
		if namen[c.Name] {
			changes = append(changes, c)
		} else {
			fehlend[c.Name] = true
		}
	}

	applied, err := s.AddShifts(r.Context(), "kw-plan:übertragen", changes)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "applied": applied, "unbekannt": sortiert(fehlend)})
}
