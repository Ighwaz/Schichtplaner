# Schichtplaner DE/IN – Go Build

## Voraussetzungen

1. **Go 1.21+** installieren: https://go.dev/dl/
2. **Wails v2** installieren:
   ```
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```
3. **WebView2** (Windows): wird meist automatisch installiert, sonst:
   https://developer.microsoft.com/en-us/microsoft-edge/webview2/

## Projekt einrichten

```bash
cd schichtplaner
go mod tidy
```

## Entwicklung (mit Hot-Reload)

```bash
wails dev
```

## Release Build (EXE)

```bash
wails build
```

→ Ergebnis: `build/bin/Schichtplaner.exe` (~10-15 MB, keine Abhängigkeiten)

## Was entsteht

- **Einzelne EXE** – kein Python, kein Browser nötig
- **Sofortiger Start** – kein Entpacken wie bei PyInstaller
- **Eigenes Fenster** – kein externer Browser
- **Datendatei** bleibt wie bisher: `schichtplan_daten.json` im gewählten Ordner
- **Config** bleibt: `~/.schichtplaner_config.json`

## Projektstruktur

```
schichtplaner/
├── main.go          # Einstiegspunkt, Wails-Setup
├── app.go           # HTTP-Router, Startup
├── data.go          # Datenstrukturen, JSON I/O
├── handlers.go      # Alle API-Handler (30 Routen)
├── holidays.go      # Feiertage DE (BW) + IN
├── ics.go           # ICS Export/Import
├── go.mod
├── wails.json
└── frontend/
    └── index.html   # Gesamtes UI (HTML/CSS/JS)
```

## Vorteile gegenüber Python/PyInstaller

| | Python + PyInstaller | Go + Wails |
|---|---|---|
| EXE-Größe | ~80 MB | ~12 MB |
| Startzeit | 3-5s (Entpacken) | <1s |
| Browser | Extern (Chrome/Edge) | Eingebettet |
| Timing-Probleme | Ja | Nein |
| Abhängigkeiten | Python-Runtime | Keine |
