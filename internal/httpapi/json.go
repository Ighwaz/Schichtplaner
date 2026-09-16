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
	"sort"
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

// fail meldet ein Problem an die Oberfläche, die es als Hinweis einblendet.
func fail(w http.ResponseWriter, err error) {
	writeJSON(w, map[string]string{"error": err.Error()})
}

// requireStore liefert die offene Ablage - oder meldet, dass kein Datenordner
// gewählt ist.
func (srv *Server) requireStore(w http.ResponseWriter) (*store.Store, bool) {
	if srv.store == nil {
		writeJSON(w, map[string]string{"error": "Kein Datenordner gewählt"})
		return nil, false
	}
	return srv.store, true
}

// data lädt den ganzen Plan - oder meldet, warum das nicht ging.
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

// write führt einen Schreibvorgang aus und meldet einen Fehlschlag, statt ihn
// zu verschlucken.
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

// uploadedFile liefert den Teil "file" eines Uploads - oder meldet den Fehler
// an die Oberfläche und gibt false zurück.
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

// bekannteNamen liefert die Namen aller eingetragenen Mitarbeiter.
//
// Schichten gibt es nur fuer Leute, die es gibt: ein Eintrag auf einen
// unbekannten Namen erschiene im Kalender als farbloser Chip, den keine
// Mitarbeiterliste mehr kennt und den niemand mehr los wird.
func (srv *Server) bekannteNamen(ctx context.Context, s *store.Store) (map[string]bool, error) {
	liste, err := s.Employees(ctx)
	if err != nil {
		return nil, err
	}
	namen := make(map[string]bool, len(liste))
	for _, m := range liste {
		namen[m.Name] = true
	}
	return namen, nil
}

// sortiert macht aus einer Menge eine geordnete Liste - fuer Meldungen, die
// immer gleich aussehen sollen.
func sortiert(menge map[string]bool) []string {
	raus := make([]string, 0, len(menge))
	for s := range menge {
		raus = append(raus, s)
	}
	sort.Strings(raus)
	return raus
}
