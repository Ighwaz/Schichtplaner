// Schichten: eintragen, austragen, Soll, Notizen und das Einfügen ganzer
// Tage. Die Regeln dahinter stehen in domain, hier steht nur der Weg dorthin.
package httpapi

import (
	"context"
	"net/http"
	"schichtplaner/internal/domain"
	"schichtplaner/internal/store"
	"strconv"
	"time"
)

// ── /api/schicht ─────────────────────────────────────────────────────────────

func (srv *Server) handleSchicht(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dates   []string `json:"dates"`
		Schicht string   `json:"schicht"`
		Name    string   `json:"name"`
		Action  string   `json:"action"`
		Force   bool     `json:"force"`
		// Replace steht, wenn der Anwender die Rückfrage "Schicht ersetzen?"
		// bestätigt hat; nur dann gibt er eine bestehende Arbeitsschicht ab.
		Replace bool `json:"replace"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}

	hols, err := srv.holidaysForDates(r.Context(), s, body.Dates)
	if err != nil {
		fail(w, err)
		return
	}
	team, err := s.Team(r.Context(), body.Name)
	if err != nil {
		fail(w, err)
		return
	}
	days := map[string]domain.DaySlot{}
	for _, date := range body.Dates {
		slot, err := s.Day(r.Context(), date)
		if err != nil {
			fail(w, err)
			return
		}
		days[date] = slot
	}

	plan := domain.PlanShifts(domain.ShiftRequest{
		Dates: body.Dates, Shift: body.Schicht, Name: body.Name,
		Action: body.Action, Force: body.Force, Replace: body.Replace,
	}, days, hols, team)

	if len(plan.Removes) > 0 {
		if _, err := s.RemoveShifts(r.Context(), "schicht:"+body.Action, plan.Removes); err != nil {
			fail(w, err)
			return
		}
	}
	if len(plan.Adds) > 0 {
		if _, err := s.AddShifts(r.Context(), "schicht:"+body.Action, plan.Adds); err != nil {
			fail(w, err)
			return
		}
	}

	// Die Oberfläche erwartet je Tag entweder den neuen Stand oder die
	// Rückfrage - diese Form ist Teil der API und bleibt unverändert.
	results := map[string]any{}
	for date, tag := range plan.Days {
		switch tag.Ask {
		case domain.AskHoliday:
			results[date] = map[string]any{
				"error": domain.AskHoliday, "holiday": tag.Holiday.Name, "country": tag.Holiday.Country,
			}
		case domain.AskReplace:
			results[date] = map[string]any{
				"error": domain.AskReplace, "blocking": tag.Blocking,
			}
		default:
			results[date] = tag.Slot
		}
	}
	warnungen := plan.Warnings
	if warnungen == nil {
		warnungen = []domain.HolidayWarning{}
	}
	writeJSON(w, map[string]any{
		"results":      results,
		"hol_warnings": warnungen,
	})
}

// holidaysForDates lädt die Feiertage aller Jahre, die die genannten Tage
// berühren.
func (srv *Server) holidaysForDates(ctx context.Context, s *store.Store, dates []string) (map[string]domain.Holiday, error) {
	customs, err := s.CustomHolidays(ctx)
	if err != nil {
		return nil, err
	}
	years := map[int]bool{time.Now().Year(): true}
	for _, date := range dates {
		if len(date) < 4 {
			continue
		}
		if y, err := strconv.Atoi(date[:4]); err == nil {
			years[y] = true
		}
	}
	hols := map[string]domain.Holiday{}
	for y := range years {
		for k, v := range domain.AllHolidays(y, customs) {
			hols[k] = v
		}
	}
	return hols, nil
}

// ── /api/soll ─────────────────────────────────────────────────────────────────

func (srv *Server) handleSoll(w http.ResponseWriter, r *http.Request) {
	var body domain.SollBesetzung
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if srv.write(w, func(s *store.Store) error { return s.SetSoll(r.Context(), body) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/notiz ────────────────────────────────────────────────────────────────

func (srv *Server) handleNotiz(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date string `json:"date"`
		Text string `json:"text"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if srv.write(w, func(s *store.Store) error { return s.SetNote(r.Context(), body.Date, body.Text) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/paste ────────────────────────────────────────────────────────────────

func (srv *Server) handlePaste(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slot  domain.DaySlot `json:"slot"`
		Dates []string       `json:"dates"`
		Mode  string         `json:"mode"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ok := srv.write(w, func(s *store.Store) error {
		if body.Mode == "replace" {
			return s.ReplaceDays(r.Context(), body.Dates, body.Slot)
		}
		return s.MergeDays(r.Context(), body.Dates, body.Slot)
	})
	if ok {
		writeJSON(w, map[string]bool{"ok": true})
	}
}
