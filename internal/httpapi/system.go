// Alles, was nicht am Plan hängt: Rückgängig-Sprung, Aenderungsverlauf,
// Datenordner und die Sicherung.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"schichtplaner/internal/domain"
	"schichtplaner/internal/store"
	"strconv"
)

// ── /api/snapshot ─────────────────────────────────────────────────────────────

func (srv *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Schichten map[string]domain.DaySlot `json:"schichten"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.Schichten == nil {
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	if srv.write(w, func(s *store.Store) error { return s.ReplaceAllShifts(r.Context(), body.Schichten) }) {
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// ── /api/history ──────────────────────────────────────────────────────────────

func (srv *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s, ok := srv.requireStore(w)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 200
	}
	entries, err := s.History(r.Context(), limit)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, entries)
}

// ── /api/holiday_coverage ─────────────────────────────────────────────────────

// handleHolidayCoverage nennt die Jahre, für die Holi und Diwali tabelliert
// sind - darüber hinaus weist die Oberfläche darauf hin, dass man sie von Hand
// nachtragen muss.
func (srv *Server) handleHolidayCoverage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]int{
		"in_movable_from": domain.MovableINFirstYear,
		"in_movable_to":   domain.MovableINLastYear,
	})
}

// ── /api/datadir ──────────────────────────────────────────────────────────────

func (srv *Server) handleGetDatadir(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"folder": srv.dataFolder,
		"file":   srv.dbPath(),
	})
}

func (srv *Server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	folder, err := srv.pickFolder(r.Context())
	if err != nil || folder == "" {
		writeJSON(w, map[string]string{"error": "Abgebrochen"})
		return
	}
	srv.useFolder(r.Context(), w, folder)
}

func (srv *Server) handleSetDatadir(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Folder string `json:"folder"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.Folder == "" {
		writeJSON(w, map[string]string{"error": "Kein Ordner"})
		return
	}
	info, err := os.Stat(body.Folder)
	if err != nil || !info.IsDir() {
		writeJSON(w, map[string]string{"error": "Ordner nicht gefunden"})
		return
	}
	srv.useFolder(r.Context(), w, body.Folder)
}

// useFolder wechselt den Datenordner und merkt ihn sich für den nächsten
// Start.
func (srv *Server) useFolder(ctx context.Context, w http.ResponseWriter, folder string) {
	if err := srv.UseFolder(ctx, folder); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "folder": folder, "file": srv.dbPath()})
}

// ── /api/export_data / import_data ───────────────────────────────────────────

func (srv *Server) handleExportData(w http.ResponseWriter, r *http.Request) {
	d, ok := srv.data(r.Context(), w)
	if !ok {
		return
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="schichtplan_backup.json"`)
	w.Write(raw)
}

func (srv *Server) handleImportData(w http.ResponseWriter, r *http.Request) {
	file, ok := uploadedFile(w, r, 32<<20)
	if !ok {
		return
	}
	defer file.Close()

	var d domain.AppData
	if err := json.NewDecoder(file).Decode(&d); err != nil {
		writeJSON(w, map[string]string{"error": "Ungültiges JSON: " + err.Error()})
		return
	}
	domain.Normalize(&d)

	if !srv.write(w, func(s *store.Store) error { return s.ReplaceAll(r.Context(), d, "import:backup") }) {
		return
	}
	writeJSON(w, map[string]any{
		"ok":          true,
		"mitarbeiter": len(d.Mitarbeiter),
		"tage":        len(d.Schichten),
		"schichten":   len(d.Schichten),
		"templates":   len(d.Templates),
	})
}

// ── /api/export_ics / import_ics are in ics.go ───────────────────────────────
