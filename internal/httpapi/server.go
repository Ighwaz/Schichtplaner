// Package httpapi bedient die Oberflaeche: es nimmt Anfragen entgegen, holt
// die Antwort bei domain und store und schreibt sie als JSON zurueck.
//
// Das Paket kennt weder Wails noch die Datenbank von innen. Was es von der
// Aussenwelt braucht - die Seite selbst und einen Ordnerdialog - bekommt es
// beim Bauen uebergeben, damit es sich ohne Fenster testen laesst.
package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"schichtplaner/internal/config"
	"schichtplaner/internal/store"
)

// PickFolderFunc fragt den Anwender nach einem Ordner. Liefert sie einen
// leeren Namen, hat er abgebrochen.
type PickFolderFunc func(ctx context.Context) (string, error)

// Server beantwortet die Anfragen der Oberflaeche.
type Server struct {
	page       []byte
	pickFolder PickFolderFunc

	dataFolder string
	store      *store.Store
	// mu reiht die Handler auf: eine Folge aus Lesen, Entscheiden und
	// Schreiben in einem Handler darf sich nicht mit einer anderen Anfrage
	// verschraenken.
	mu sync.Mutex
}

// New baut den Server. page ist die auszuliefernde Oberflaeche, pickFolder der
// Ordnerdialog des Fensters; ohne Dialog meldet der entsprechende Weg einen
// Abbruch, was das Testen ohne Fenster erlaubt.
func New(page []byte, pickFolder PickFolderFunc) *Server {
	if pickFolder == nil {
		pickFolder = func(context.Context) (string, error) { return "", nil }
	}
	return &Server{page: page, pickFolder: pickFolder}
}

// UseFolder oeffnet den Datenordner und merkt ihn sich fuer den naechsten
// Start. Ist keiner nutzbar, bleibt der Server ohne Ablage - die Oberflaeche
// zeigt dann "Kein Datenordner gewaehlt" statt gar nicht zu starten.
func (s *Server) UseFolder(ctx context.Context, folder string) error {
	if err := s.setDataFolder(ctx, folder); err != nil {
		return err
	}
	cfg := config.Load()
	cfg.DataFolder = folder
	return config.Save(cfg)
}

// Close gibt die Datenbank frei.
func (s *Server) Close() error { return s.store.Close() }

// setDataFolder opens the database in folder and replaces any open one. The
// new database is opened first: if that fails, the previously opened folder
// stays in use instead of leaving the app without any data at all.
func (srv *Server) setDataFolder(ctx context.Context, folder string) error {
	if folder == "" {
		srv.store.Close()
		srv.store = nil
		srv.dataFolder = ""
		return nil
	}
	neu, err := store.Open(ctx, folder)
	if err != nil {
		return err
	}
	srv.store.Close()
	srv.store = neu
	srv.dataFolder = folder
	return nil
}

// dbPath is the database file, shown in the folder bar of the UI.
func (srv *Server) dbPath() string {
	if srv.dataFolder == "" {
		return ""
	}
	return srv.store.Path()
}

// ServeHTTP handles all requests from the Wails WebView
func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Die Oberflaeche selbst
	if path == "/" || path == "/index.html" || path == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(srv.page)
		return
	}

	// API routes - serialized, see App.mu
	srv.mu.Lock()
	defer srv.mu.Unlock()

	switch {
	case path == "/api/data":
		srv.handleGetData(w, r)
	case path == "/api/snapshot" && r.Method == http.MethodPost:
		srv.handleSnapshot(w, r)
	case path == "/api/mitarbeiter" && r.Method == http.MethodGet:
		srv.handleGetMitarbeiter(w, r)
	case path == "/api/mitarbeiter" && r.Method == http.MethodPost:
		srv.handleAddMitarbeiter(w, r)
	case path == "/api/mitarbeiter/bulk" && r.Method == http.MethodPost:
		srv.handleBulkMitarbeiter(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && strings.HasSuffix(path, "/color"):
		srv.handleSetColor(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && strings.HasSuffix(path, "/prefs"):
		srv.handleSetPrefs(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && strings.HasSuffix(path, "/restore"):
		srv.handleRestoreMitarbeiter(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && r.Method == http.MethodPut:
		srv.handleUpdateMitarbeiter(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && r.Method == http.MethodDelete:
		srv.handleDeleteMitarbeiter(w, r)
	case path == "/api/schicht" && r.Method == http.MethodPost:
		srv.handleSchicht(w, r)
	case path == "/api/soll" && r.Method == http.MethodPost:
		srv.handleSoll(w, r)
	case path == "/api/notiz" && r.Method == http.MethodPost:
		srv.handleNotiz(w, r)
	case path == "/api/paste" && r.Method == http.MethodPost:
		srv.handlePaste(w, r)
	case path == "/api/holiday_coverage":
		srv.handleHolidayCoverage(w, r)
	case strings.HasPrefix(path, "/api/holidays/"):
		srv.handleHolidays(w, r)
	case path == "/api/custom_holidays" && r.Method == http.MethodGet:
		srv.handleGetCustomHolidays(w, r)
	case path == "/api/custom_holidays/bulk" && r.Method == http.MethodPost:
		srv.handleBulkCustomHolidays(w, r)
	case path == "/api/custom_holidays" && r.Method == http.MethodPost:
		srv.handleAddCustomHoliday(w, r)
	case strings.HasPrefix(path, "/api/custom_holidays/") && r.Method == http.MethodDelete:
		srv.handleDeleteCustomHoliday(w, r)
	case path == "/api/templates" && r.Method == http.MethodGet:
		srv.handleGetTemplates(w, r)
	case path == "/api/templates" && r.Method == http.MethodPost:
		srv.handleSaveTemplate(w, r)
	case strings.HasPrefix(path, "/api/templates/") && r.Method == http.MethodDelete:
		srv.handleDeleteTemplate(w, r)
	case path == "/api/autoplan" && r.Method == http.MethodPost:
		srv.handleAutoplan(w, r)
	case path == "/api/ruf_kw" && r.Method == http.MethodGet:
		srv.handleGetRufKW(w, r)
	case path == "/api/ruf_kw" && r.Method == http.MethodPost:
		srv.handleSaveRufKW(w, r)
	case path == "/api/ruf_kw/apply" && r.Method == http.MethodPost:
		srv.handleApplyRufKW(w, r)
	case path == "/api/history":
		srv.handleHistory(w, r)
	case path == "/api/datadir":
		srv.handleGetDatadir(w, r)
	case path == "/api/pick_folder" && r.Method == http.MethodPost:
		srv.handlePickFolder(w, r)
	case path == "/api/set_datadir" && r.Method == http.MethodPost:
		srv.handleSetDatadir(w, r)
	case strings.HasPrefix(path, "/api/export_ics"):
		srv.handleExportICS(w, r)
	case path == "/api/import_ics" && r.Method == http.MethodPost:
		srv.handleImportICS(w, r)
	case path == "/api/export_data":
		srv.handleExportData(w, r)
	case path == "/api/import_data" && r.Method == http.MethodPost:
		srv.handleImportData(w, r)
	default:
		http.NotFound(w, r)
	}
}
