// Templates und der Autoplan, der sie auf einen Monat anwendet.
package httpapi

import (
	"net/http"
	"schichtplaner/internal/domain"
	"schichtplaner/internal/store"
)

// ── /api/templates ───────────────────────────────────────────────────────────

func (srv *Server) handleGetTemplates(w http.ResponseWriter, r *http.Request) {
	if d, ok := srv.data(r.Context(), w); ok {
		writeJSON(w, d.Templates)
	}
}

func (srv *Server) handleSaveTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string          `json:"name"`
		Template domain.Template `json:"template"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.Name == "" {
		writeJSON(w, map[string]string{"error": "Name erforderlich"})
		return
	}
	if srv.write(w, func(s *store.Store) error { return s.SaveTemplate(r.Context(), body.Name, body.Template) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (srv *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	name := pathSegment(r.URL.Path, "/api/templates")
	if srv.write(w, func(s *store.Store) error { return s.DeleteTemplate(r.Context(), name) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/autoplan ─────────────────────────────────────────────────────────────

func (srv *Server) handleAutoplan(w http.ResponseWriter, r *http.Request) {
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
	d, ok := srv.data(r.Context(), w)
	if !ok {
		return
	}
	tmpl, exists := d.Templates[body.Template]
	if !exists {
		writeJSON(w, map[string]string{"error": "domain.Template nicht gefunden"})
		return
	}

	teams := map[string]string{}
	for _, m := range d.Mitarbeiter {
		teams[m.Name] = m.Team
	}
	plan := domain.ApplyTemplate(tmpl, body.Year, body.Month, d.Schichten,
		domain.AllHolidays(body.Year, d.CustomHolidays), teams)

	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	planned, err := s.AddShifts(r.Context(), "autoplan:"+body.Template, plan.Changes)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"ok":               true,
		"planned":          planned,
		"skipped_holiday":  plan.SkippedHoliday,
		"skipped_conflict": plan.SkippedConflict,
	})
}
