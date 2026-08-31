package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App holds all application state
type App struct {
	ctx        context.Context
	dataFolder string
	store      *Store
	// mu serializes the API handlers so that a read-decide-write sequence
	// inside one handler cannot interleave with another request.
	mu sync.Mutex
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg := loadConfig()
	if cfg.DataFolder != "" {
		if info, err := os.Stat(cfg.DataFolder); err == nil && info.IsDir() {
			if err := a.setDataFolder(cfg.DataFolder); err == nil {
				return
			}
			runtime.LogErrorf(ctx, "Datenordner %s nicht nutzbar", cfg.DataFolder)
		}
	}

	folder, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Datenordner für Schichtplaner wählen",
	})
	if err != nil || folder == "" {
		exe, _ := os.Executable()
		folder = filepath.Dir(exe)
	}
	if err := a.setDataFolder(folder); err != nil {
		runtime.LogError(ctx, err.Error())
		return
	}
	cfg.DataFolder = folder
	saveConfig(cfg)
}

func (a *App) domReady(ctx context.Context) {}

func (a *App) shutdown(ctx context.Context) {
	a.store.Close()
}

// setDataFolder opens the database in folder and replaces any open one. The
// new database is opened first: if that fails, the previously opened folder
// stays in use instead of leaving the app without any data at all.
func (a *App) setDataFolder(folder string) error {
	if folder == "" {
		a.store.Close()
		a.store = nil
		a.dataFolder = ""
		return nil
	}
	store, err := openStore(folder)
	if err != nil {
		return err
	}
	a.store.Close()
	a.store = store
	a.dataFolder = folder
	return nil
}

// dbPath is the database file, shown in the folder bar of the UI.
func (a *App) dbPath() string {
	if a.dataFolder == "" {
		return ""
	}
	return filepath.Join(a.dataFolder, dbFileName)
}

// ServeHTTP handles all requests from the Wails WebView
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Serve the main HTML
	if path == "/" || path == "/index.html" || path == "" {
		data, err := assets.ReadFile("frontend/index.html")
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return
	}

	// API routes - serialized, see App.mu
	a.mu.Lock()
	defer a.mu.Unlock()

	switch {
	case path == "/api/data":
		a.handleGetData(w, r)
	case path == "/api/snapshot" && r.Method == http.MethodPost:
		a.handleSnapshot(w, r)
	case path == "/api/mitarbeiter" && r.Method == http.MethodGet:
		a.handleGetMitarbeiter(w, r)
	case path == "/api/mitarbeiter" && r.Method == http.MethodPost:
		a.handleAddMitarbeiter(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && strings.HasSuffix(path, "/color"):
		a.handleSetColor(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && strings.HasSuffix(path, "/prefs"):
		a.handleSetPrefs(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && strings.HasSuffix(path, "/restore"):
		a.handleRestoreMitarbeiter(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && r.Method == http.MethodPut:
		a.handleUpdateMitarbeiter(w, r)
	case strings.HasPrefix(path, "/api/mitarbeiter/") && r.Method == http.MethodDelete:
		a.handleDeleteMitarbeiter(w, r)
	case path == "/api/schicht" && r.Method == http.MethodPost:
		a.handleSchicht(w, r)
	case path == "/api/soll" && r.Method == http.MethodPost:
		a.handleSoll(w, r)
	case path == "/api/notiz" && r.Method == http.MethodPost:
		a.handleNotiz(w, r)
	case path == "/api/paste" && r.Method == http.MethodPost:
		a.handlePaste(w, r)
	case path == "/api/holiday_coverage":
		a.handleHolidayCoverage(w, r)
	case strings.HasPrefix(path, "/api/holidays/"):
		a.handleHolidays(w, r)
	case path == "/api/custom_holidays" && r.Method == http.MethodGet:
		a.handleGetCustomHolidays(w, r)
	case path == "/api/custom_holidays" && r.Method == http.MethodPost:
		a.handleAddCustomHoliday(w, r)
	case strings.HasPrefix(path, "/api/custom_holidays/") && r.Method == http.MethodDelete:
		a.handleDeleteCustomHoliday(w, r)
	case path == "/api/templates" && r.Method == http.MethodGet:
		a.handleGetTemplates(w, r)
	case path == "/api/templates" && r.Method == http.MethodPost:
		a.handleSaveTemplate(w, r)
	case strings.HasPrefix(path, "/api/templates/") && r.Method == http.MethodDelete:
		a.handleDeleteTemplate(w, r)
	case path == "/api/autoplan" && r.Method == http.MethodPost:
		a.handleAutoplan(w, r)
	case path == "/api/ruf_kw" && r.Method == http.MethodGet:
		a.handleGetRufKW(w, r)
	case path == "/api/ruf_kw" && r.Method == http.MethodPost:
		a.handleSaveRufKW(w, r)
	case path == "/api/ruf_kw/apply" && r.Method == http.MethodPost:
		a.handleApplyRufKW(w, r)
	case path == "/api/history":
		a.handleHistory(w, r)
	case path == "/api/datadir":
		a.handleGetDatadir(w, r)
	case path == "/api/pick_folder" && r.Method == http.MethodPost:
		a.handlePickFolder(w, r)
	case path == "/api/set_datadir" && r.Method == http.MethodPost:
		a.handleSetDatadir(w, r)
	case strings.HasPrefix(path, "/api/export_ics"):
		a.handleExportICS(w, r)
	case path == "/api/import_ics" && r.Method == http.MethodPost:
		a.handleImportICS(w, r)
	case path == "/api/export_data":
		a.handleExportData(w, r)
	case path == "/api/import_data" && r.Method == http.MethodPost:
		a.handleImportData(w, r)
	default:
		http.NotFound(w, r)
	}
}
