# Schichtplaner DE/IN

Desktop-Schichtplanung für ein Team, das über Deutschland (Baden-Württemberg)
und Indien verteilt arbeitet. Go + [Wails v2](https://wails.io) mit einer
Single-File-Weboberfläche – eine EXE, keine Runtime-Abhängigkeiten.

## Funktionen

- **Kalender** mit Früh-, Spät- und Rufbereitschaftsschichten sowie Urlaub,
  Krank, Elternzeit und Sonderurlaub
- **Feiertage** DE (BW) und IN inkl. Brückentagen und eigenen Feiertagen.
  Ein Eintrag am Feiertag des eigenen Teams wird abgefragt, nicht verhindert
- **Konfliktprüfung**: wer schon in einer anderen Arbeitsschicht steht, wird
  nur nach Rückfrage umgetragen – Rufbereitschaft bleibt dabei erhalten
- **Templates** pro Wochentag und Person, per Autoplan auf einen Monat anwendbar
- **Rufbereitschaft nach KW** – Wochenplan, der auf einen Monat oder ein ganzes
  Jahr übertragen werden kann
- **Import/Export** als ICS (Kalender) und JSON (vollständiges Backup)
- **Datenordner frei wählbar**

## Datenhaltung

Der Plan liegt als **SQLite-Datenbank** `schichtplan.db` im gewählten
Datenordner. Jede Änderung läuft in einer Transaktion und schreibt nur die
betroffenen Zeilen – ein Klick im Kalender rührt nicht den ganzen Bestand an.

| Tabelle | Inhalt |
|---|---|
| `employees` | Mitarbeiter mit Team, Farbe, Icon, Wunsch-Schichten |
| `shifts` | je Zeile ein Eintrag `(Datum, Schicht, Name)` |
| `notes` | Tagesnotizen |
| `custom_holidays` | eigene Feiertage |
| `templates` | Wochen-Templates |
| `ruf_kw` | Rufbereitschaftsplan je Kalenderwoche |
| `settings` | Soll-Besetzung, Migrationsmarker |
| `changelog` | Änderungsprotokoll mit Zeitstempel |

Eine vorhandene `schichtplan_daten.json` aus älteren Versionen wird beim ersten
Start **einmalig** in eine noch leere Datenbank übernommen; danach bleibt sie
unangetastet liegen und dient als Sicherungskopie. Ein Backup als JSON gibt es
weiterhin über den Export.

Die Ordner-Einstellung steht in `~/.schichtplaner_config.json`.

**Nicht auf ein Netzlaufwerk legen.** SQLite verlässt sich auf Dateisperren,
die über SMB unzuverlässig sind. Für mehrere Leute gleichzeitig wäre der Weg,
das Backend als Dienst auf einem Rechner laufen zu lassen – die App ist bereits
ein HTTP-Server, das Fenster nur der Browser davor.

## Entwicklung

Voraussetzungen: Go 1.25+, [Wails v2](https://wails.io/docs/gettingstarted/installation),
unter Windows WebView2.

```bash
go mod tidy
wails dev      # Hot-Reload
wails build    # -> build/bin/Schichtplaner.exe
go test ./...  # API- und Speichertests
```

Details zum Build siehe [BUILD.md](BUILD.md).

## Aufbau

```
main.go           Einstiegspunkt, Wails-Setup
app.go            HTTP-Router, Startup, Datenordner
data.go           Datenstrukturen und Slot-Helfer
store.go          SQLite-Speicher, Migration, Änderungsprotokoll
handlers.go       API-Handler
holidays.go       Feiertage DE (BW) + IN, Brückentage
ics.go            ICS Export/Import
handlers_test.go  Tests gegen die API-Verträge des Frontends
frontend/
  index.html      gesamtes UI (HTML/CSS/JS)
```

Das Frontend spricht das Backend ausschließlich über `/api/…` an; die Routen
sind in [app.go](app.go) gebündelt. `GET /api/history?limit=200` liefert das
Änderungsprotokoll.

## Bekannte Grenzen

- Indische Feiertage mit beweglichem Datum (Holi, Diwali) sind bis
  einschließlich 2028 hinterlegt und müssen danach in
  [holidays.go](holidays.go) ergänzt werden.
- Das Änderungsprotokoll hält fest, *was* wann geändert wurde, nicht *wer* –
  die App kennt keine Benutzer.
