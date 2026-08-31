# Schichtplaner DE/IN – Go Build

## Voraussetzungen

1. **Go 1.25+** installieren: https://go.dev/dl/
2. **Wails v2** installieren:
   ```
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```
3. **WebView2** (Windows): wird meist automatisch installiert, sonst:
   https://developer.microsoft.com/en-us/microsoft-edge/webview2/

Ein C-Compiler wird **nicht** gebraucht: SQLite kommt über
`modernc.org/sqlite` als reines Go.

## Projekt einrichten

```bash
go mod tidy
```

## Entwicklung (mit Hot-Reload)

```bash
wails dev
```

## Tests

```bash
go test ./...
```

Die Tests fahren den HTTP-Router gegen eine Wegwerf-Datenbank und prüfen die
Verträge, die das Frontend erwartet – Konfliktregeln, KW-Plan, Umbenennen,
Löschen/Wiederherstellen, ICS- und Backup-Export sowie die einmalige Migration
der alten JSON-Datei.

## Release Build (EXE)

```bash
wails build
```

→ Ergebnis: `build/bin/Schichtplaner.exe` (~16 MB, keine Abhängigkeiten)

## Was entsteht

- **Einzelne EXE** – kein Python, kein Browser nötig
- **Sofortiger Start** – kein Entpacken wie bei PyInstaller
- **Eigenes Fenster** – kein externer Browser
- **Daten** in `schichtplan.db` (SQLite) im gewählten Ordner
- **Config** in `~/.schichtplaner_config.json`

## Projektstruktur

```
schichtplaner/
├── main.go            # Einstiegspunkt, Wails-Setup
├── app.go             # HTTP-Router, Startup, Datenordner
├── data.go            # Datenstrukturen, Slot-Helfer
├── store.go           # SQLite-Speicher, Migration, Änderungsprotokoll
├── handlers.go        # API-Handler
├── holidays.go        # Feiertage DE (BW) + IN
├── ics.go             # ICS Export/Import
├── handlers_test.go   # Tests
├── go.mod
├── wails.json
└── frontend/
    └── index.html     # Gesamtes UI (HTML/CSS/JS)
```

## Vorteile gegenüber Python/PyInstaller

| | Python + PyInstaller | Go + Wails |
|---|---|---|
| EXE-Größe | ~80 MB | ~16 MB |
| Startzeit | 3-5s (Entpacken) | <1s |
| Browser | Extern (Chrome/Edge) | Eingebettet |
| Timing-Probleme | Ja | Nein |
| Abhängigkeiten | Python-Runtime | Keine |
