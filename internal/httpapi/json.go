// Kleinkram, den jeder Handler braucht: JSON hinein und hinaus, der Griff
// zur Ablage und das Melden von Fehlern.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"schichtplaner/internal/domain"
	"schichtplaner/internal/store"
	"strings"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("Anfrage nicht lesbar: %w", err)
	}
	return nil
}

// fail reports a problem to the frontend, which shows it as a toast.
func fail(w http.ResponseWriter, err error) {
	writeJSON(w, map[string]string{"error": err.Error()})
}

// requireStore returns the open data store, or reports that no folder is set.
func (srv *Server) requireStore(w http.ResponseWriter) (*store.Store, bool) {
	if srv.store == nil {
		writeJSON(w, map[string]string{"error": "Kein Datenordner gewählt"})
		return nil, false
	}
	return srv.store, true
}

// data loads the whole plan, or reports why it could not be read.
func (srv *Server) data(ctx context.Context, w http.ResponseWriter) (domain.AppData, bool) {
	s, ok := srv.requireStore(w)
	if !ok {
		return domain.AppData{}, false
	}
	d, err := s.Load(ctx)
	if err != nil {
		fail(w, err)
		return domain.AppData{}, false
	}
	return d, true
}

// write runs one store operation and reports a failure instead of swallowing it.
func (srv *Server) write(w http.ResponseWriter, fn func(*store.Store) error) bool {
	s, ok := srv.requireStore(w)
	if !ok {
		return false
	}
	if err := fn(s); err != nil {
		fail(w, err)
		return false
	}
	return true
}

// uploadedFile returns the "file" part of a multipart upload, or reports the
// error to the client and returns false.
func uploadedFile(w http.ResponseWriter, r *http.Request, maxMemory int64) (multipart.File, bool) {
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		writeJSON(w, map[string]string{"error": "Ungültiger Upload"})
		return nil, false
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, map[string]string{"error": "Keine Datei"})
		return nil, false
	}
	return file, true
}

func pathSegment(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimPrefix(s, "/")
	s, _ = url.PathUnescape(s)
	return s
}
