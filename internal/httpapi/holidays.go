// Feiertage: die berechneten des Jahres und die selbst eingetragenen.
package httpapi

import (
	"net/http"
	"schichtplaner/internal/domain"
	"schichtplaner/internal/store"
	"strconv"
	"strings"
	"time"
)

// ── /api/holidays/<year> ─────────────────────────────────────────────────────

func (srv *Server) handleHolidays(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(pathSegment(r.URL.Path, "/api/holidays"))
	if err != nil {
		http.Error(w, "invalid year", 400)
		return
	}
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	customs, err := s.CustomHolidays(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, domain.AllHolidays(year, customs))
}

// ── /api/custom_holidays ─────────────────────────────────────────────────────

func (srv *Server) handleGetCustomHolidays(w http.ResponseWriter, r *http.Request) {
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	list, err := s.CustomHolidays(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, list)
}

func (srv *Server) handleAddCustomHoliday(w http.ResponseWriter, r *http.Request) {
	var body domain.CustomHoliday
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if srv.write(w, func(s *store.Store) error { return s.AddCustomHoliday(r.Context(), body) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// handleBulkCustomHolidays creates a whole list of holidays at once.
func (srv *Server) handleBulkCustomHolidays(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Feiertage []domain.CustomHoliday `json:"feiertage"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var list []domain.CustomHoliday
	seen := map[string]bool{}
	for _, ch := range body.Feiertage {
		ch.Date = strings.TrimSpace(ch.Date)
		ch.Name = strings.TrimSpace(ch.Name)
		if ch.Name == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", ch.Date); err != nil {
			continue
		}
		if ch.Country == "" {
			ch.Country = "DE"
		}
		key := ch.Date + "|" + ch.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		list = append(list, ch)
	}
	if len(list) == 0 {
		writeJSON(w, map[string]string{"error": "Keine gültigen Feiertage"})
		return
	}
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	added, skipped, err := s.AddCustomHolidays(r.Context(), list)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "angelegt": added, "uebersprungen": skipped})
}

func (srv *Server) handleDeleteCustomHoliday(w http.ResponseWriter, r *http.Request) {
	key := pathSegment(r.URL.Path, "/api/custom_holidays")
	parts := strings.SplitN(key, "|", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid key", 400)
		return
	}
	if srv.write(w, func(s *store.Store) error { return s.DeleteCustomHoliday(r.Context(), parts[0], parts[1]) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}
