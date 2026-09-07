// Schichtplaner DE/IN - Schichtplanung für ein Team, das über Deutschland und
// Indien verteilt arbeitet.
//
// Diese Datei ist die Verdrahtung und sonst nichts: sie packt die Oberfläche
// ins Programm, baut den Server aus internal/httpapi und öffnet das Fenster.
// Sie ist der einzige Ort, an dem Wails vorkommt - alles unter internal/ läuft
// auch ohne Fenster und ist deshalb ohne Fenster prüfbar.
//
// Das Hauptpaket liegt im Wurzelverzeichnis, weil `wails build` das Modul im
// aktuellen Verzeichnis übersetzt und den Ordner aus wails.json (frontend/)
// danebenliegend erwartet. Siehe README, Abschnitt "Aufbau".
package main

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"schichtplaner/internal/config"
	"schichtplaner/internal/httpapi"
)

//go:embed frontend/index.html
var page []byte

func main() {
	srv := httpapi.New(page, ordnerDialog)

	err := wails.Run(&options.App{
		Title:            "Schichtplaner DE/IN",
		Width:            1440,
		Height:           900,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 255},
		OnStartup:        func(ctx context.Context) { starte(ctx, srv) },
		OnShutdown:       func(context.Context) { srv.Close() },
		AssetServer: &assetserver.Options{
			Handler: http.HandlerFunc(srv.ServeHTTP),
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		log.Fatalf("Fenster konnte nicht geöffnet werden: %v", err)
	}
}

// ordnerDialog fragt den Anwender nach einem Ordner - der einzige Weg, auf dem
// httpapi das Fenster braucht, und darum als Funktion hineingereicht.
func ordnerDialog(ctx context.Context) (string, error) {
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Datenordner wählen",
	})
}

// starte sucht den Datenordner: erst den zuletzt benutzten, dann den
// gewählten, zuletzt den Ordner des Programms. Ein fehlgeschlagener Versuch
// verhindert den Start nicht - die Oberfläche meldet dann, dass kein Ordner
// gewählt ist, statt gar nicht erst zu erscheinen.
func starte(ctx context.Context, srv *httpapi.Server) {
	if cfg := config.Load(); cfg.DataFolder != "" {
		if info, err := os.Stat(cfg.DataFolder); err == nil && info.IsDir() {
			if err := srv.UseFolder(ctx, cfg.DataFolder); err == nil {
				return
			}
			runtime.LogErrorf(ctx, "Datenordner %s nicht nutzbar", cfg.DataFolder)
		}
	}

	folder, err := ordnerDialog(ctx)
	if err != nil || folder == "" {
		exe, _ := os.Executable()
		folder = filepath.Dir(exe)
	}
	if err := srv.UseFolder(ctx, folder); err != nil {
		runtime.LogErrorf(ctx, "Datenordner %s: %v", folder, err)
	}
}
