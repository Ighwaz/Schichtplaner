// Mitarbeiter: anlegen, umbenennen, loeschen, wiederherstellen, Farbe und
// Praeferenzen - und der Gesamtabzug des Plans.
package httpapi

import (
	"context"
	"net/http"
	"schichtplaner/internal/domain"
	"schichtplaner/internal/store"
	"strings"
)

// ── /api/data ─────────────────────────────────────────────────────────────────

func (srv *Server) handleGetData(w http.ResponseWriter, r *http.Request) {
	if d, ok := srv.data(r.Context(), w); ok {
		writeJSON(w, d)
	}
}

// ── /api/mitarbeiter ─────────────────────────────────────────────────────────

// respondEmployees answers with the current employee list plus optional extras.
func (srv *Server) respondEmployees(ctx context.Context, w http.ResponseWriter, extra map[string]any) {
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	list, err := s.Employees(ctx)
	if err != nil {
		fail(w, err)
		return
	}
	out := map[string]any{"ok": true, "mitarbeiter": list}
	for k, v := range extra {
		out[k] = v
	}
	writeJSON(w, out)
}

func (srv *Server) handleGetMitarbeiter(w http.ResponseWriter, r *http.Request) {
	if d, ok := srv.data(r.Context(), w); ok {
		writeJSON(w, d.Mitarbeiter)
	}
}

func (srv *Server) handleAddMitarbeiter(w http.ResponseWriter, r *http.Request) {
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
	d, ok := srv.data(r.Context(), w)
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
	if !srv.write(w, func(s *store.Store) error {
		return s.AddEmployee(r.Context(), domain.Employee{Name: name, Team: body.Team, Color: color, Prefs: map[string]string{}})
	}) {
		return
	}
	srv.respondEmployees(r.Context(), w, nil)
}

// handleBulkMitarbeiter creates a whole list of employees at once.
func (srv *Server) handleBulkMitarbeiter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mitarbeiter []struct {
			Name  string `json:"name"`
			Team  string `json:"team"`
			Color string `json:"color"`
		} `json:"mitarbeiter"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var list []domain.Employee
	seen := map[string]bool{}
	for _, m := range body.Mitarbeiter {
		name := strings.TrimSpace(m.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		color := m.Color
		if color == "" {
			color = "#4a9eff"
		}
		list = append(list, domain.Employee{Name: name, Team: m.Team, Color: color, Prefs: map[string]string{}})
	}
	if len(list) == 0 {
		writeJSON(w, map[string]string{"error": "Keine gültigen Namen"})
		return
	}
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	added, skipped, err := s.AddEmployees(r.Context(), list)
	if err != nil {
		fail(w, err)
		return
	}
	srv.respondEmployees(r.Context(), w, map[string]any{"angelegt": added, "uebersprungen": skipped})
}

func (srv *Server) handleUpdateMitarbeiter(w http.ResponseWriter, r *http.Request) {
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
	d, ok := srv.data(r.Context(), w)
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
	if !srv.write(w, func(s *store.Store) error {
		return s.UpdateEmployee(r.Context(), oldName, domain.Employee{
			Name: body.Name, Team: body.Team, Color: body.Color, Icon: body.Icon,
		})
	}) {
		return
	}
	srv.respondEmployees(r.Context(), w, nil)
}

func (srv *Server) handleDeleteMitarbeiter(w http.ResponseWriter, r *http.Request) {
	name := pathSegment(r.URL.Path, "/api/mitarbeiter")
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	// The backup lets the frontend offer a restore when the name is re-added -
	// including team, colour and icon, not just the shift entries.
	gone, backup, err := s.DeleteEmployee(r.Context(), name)
	if err != nil {
		fail(w, err)
		return
	}
	srv.respondEmployees(r.Context(), w, map[string]any{"backup": backup, "employee": gone})
}

func (srv *Server) handleSetColor(w http.ResponseWriter, r *http.Request) {
	// /api/mitarbeiter/<name>/color
	name := pathSegment(strings.TrimSuffix(r.URL.Path, "/color"), "/api/mitarbeiter")
	var body struct {
		Color string `json:"color"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if srv.write(w, func(s *store.Store) error { return s.SetColor(r.Context(), name, body.Color) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (srv *Server) handleSetPrefs(w http.ResponseWriter, r *http.Request) {
	name := pathSegment(strings.TrimSuffix(r.URL.Path, "/prefs"), "/api/mitarbeiter")
	var body struct {
		Prefs map[string]string `json:"prefs"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if srv.write(w, func(s *store.Store) error { return s.SetPrefs(r.Context(), name, body.Prefs) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (srv *Server) handleRestoreMitarbeiter(w http.ResponseWriter, r *http.Request) {
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
	var changes []domain.ShiftChange
	for date, shifts := range body.Entries {
		for shift, on := range shifts {
			if on {
				changes = append(changes, domain.ShiftChange{Date: date, Shift: shift, Name: body.Name})
			}
		}
	}
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	restored, err := s.AddShifts(r.Context(), "mitarbeiter:wiederherstellen", changes)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "restored": restored})
}
