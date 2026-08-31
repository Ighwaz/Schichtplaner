# Schichtplaner DE/IN

Desktop-Schichtplanung für ein Team, das über Deutschland (Baden-Württemberg)
und Indien verteilt arbeitet. Go + [Wails v2](https://wails.io) mit einer
Single-File-Weboberfläche – eine EXE, keine Runtime-Abhängigkeiten.

## Funktionen

- **Kalender** mit Früh-, Spät- und Rufbereitschaftsschichten sowie Urlaub,
  Krank, Elternzeit und Sonderurlaub
- **Feiertage** DE (BW) und IN inkl. Brückentagen und eigenen Feiertagen;
  Einträge an Feiertagen des jeweiligen Teams werden gewarnt
- **Templates** pro Wochentag und Person, per Autoplan auf einen Monat anwendbar
- **Rufbereitschaft nach KW** – Wochenplan, der auf einen Monat oder ein ganzes
  Jahr übertragen werden kann
- **Import/Export** als ICS (Kalender) und JSON (vollständiges Backup)
- **Datenordner frei wählbar** – z. B. ein Netzlaufwerk für das ganze Team

## Daten

| Was | Wo |
|---|---|
| Schichtdaten | `schichtplan_daten.json` im gewählten Datenordner |
| Ordner-Einstellung | `~/.schichtplaner_config.json` |

Geschrieben wird über eine temporäre Datei mit anschließendem Rename, damit ein
Absturz während des Speicherns die Daten nicht abschneidet.

## Entwicklung

Voraussetzungen: Go 1.22+, [Wails v2](https://wails.io/docs/gettingstarted/installation),
unter Windows WebView2.

```bash
go mod tidy
wails dev      # Hot-Reload
wails build    # -> build/bin/Schichtplaner.exe
go test ./...  # API-Tests
```

Details zum Build siehe [BUILD.md](BUILD.md).

## Aufbau

```
main.go           Einstiegspunkt, Wails-Setup
app.go            HTTP-Router, Startup, Datenordner
data.go           Datenstrukturen, JSON I/O, Migrationen
handlers.go       API-Handler
holidays.go       Feiertage DE (BW) + IN, Brückentage
ics.go            ICS Export/Import
handlers_test.go  Tests gegen die API-Verträge des Frontends
frontend/
  index.html      gesamtes UI (HTML/CSS/JS)
```

Das Frontend spricht das Backend ausschließlich über `/api/...` an; die Routen
sind in [app.go](app.go) gebündelt.

## Bekannte Grenzen

- Indische Feiertage mit beweglichem Datum (Holi, Diwali) sind bis
  einschließlich 2028 hinterlegt und müssen danach in
  [holidays.go](holidays.go) ergänzt werden.
- Der Datenordner ist für gleichzeitige Bearbeitung durch mehrere Instanzen
  nicht gesperrt – ein Client nach dem anderen.
