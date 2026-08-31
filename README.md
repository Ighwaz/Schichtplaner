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
- **Massenanlage** für Mitarbeiter und eigene Feiertage: Liste einfügen
  (auch aus Excel), Vorschau prüfen, anlegen
- **Import/Export** als ICS (Kalender) und JSON (vollständiges Backup)
- **Datenordner frei wählbar**

## Datenhaltung

Der Plan liegt als **SQLite-Datenbank** `schichtplan.db` im gewählten
Datenordner – eine einzige Datei, die nach jedem Schreibvorgang vollständig
ist. Jede Änderung läuft in einer Transaktion und schreibt nur die betroffenen
Zeilen; ein Klick im Kalender rührt nicht den ganzen Bestand an. Gemessen mit
fünf Jahresplänen (13.000 Einträge): kompletter Ladevorgang 14 ms, ein Klick
3 ms.

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

## Feiertage

Gesetzliche Feiertage DE (Baden-Württemberg) werden gerechnet, die indischen
Fixtermine ebenso. **Holi und Diwali** folgen dem lunisolaren Kalender und
haben keine Formel – sie stehen als Tabelle in [holidays.go](holidays.go) und
reichen derzeit bis **2036** (Quelle: qppstudio.net, jeweils der Tag, den
Indien als Feiertag begeht – bei Holi also Rangwali Holi, nicht der Holika
Dahan am Abend davor).

Für Jahre jenseits der Tabelle weist die Ansicht „Eigene Feiertage" darauf hin.
Bis die Tabelle verlängert wird, lassen sich beide als eigener Feiertag mit
Land `IN` nachtragen – ein eigener Feiertag verhält sich genau wie ein
gesetzlicher und fragt vor einem Eintrag nach.

`GET /api/holiday_coverage` nennt den abgedeckten Zeitraum.

Mehrere Feiertage auf einmal legt der Knopf **⇊ Liste** an – eine Zeile je Tag,
`Datum;Name;Land`. Für Mitarbeiter gibt es denselben Knopf in der Seitenleiste.

## Bekannte Grenzen

- Das Änderungsprotokoll hält fest, *was* wann geändert wurde, nicht *wer* –
  die App kennt keine Benutzer.
- Brückentage werden nur um gesetzliche Feiertage herum erkannt, nicht um
  eigene.
