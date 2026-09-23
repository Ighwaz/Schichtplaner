# Schichtplaner DE/IN

> **Branch `schlank`.** Diese Fassung plant nur Schichten: Früh, Spät und
> Rufbereitschaft. Urlaub, Krank, Elternzeit und Sonderurlaub gibt es nicht
> mehr. Vorhandene Einträge dieser Art bleiben unangetastet in der Datenbank
> liegen – ein Wechsel zurück auf `main` bringt sie zurück.

Desktop-Schichtplanung für ein Team, das über Deutschland (Baden-Württemberg)
und Indien verteilt arbeitet. Go + [Wails v2](https://wails.io) mit einer
Single-File-Weboberfläche – eine EXE, keine Runtime-Abhängigkeiten.

## Funktionen

- **Vier Schichten**: Früh, Normaldienst (Gleitzeit, zeitlich zwischen Früh und
  Spät), Spät – die drei schließen sich gegenseitig aus – sowie Rufbereitschaft,
  die immer zusätzlich dazu läuft und nie nachfragt
- **Kalender**: jeder Tag zeigt immer alle Schichten in derselben
  Reihenfolge, mit Besetzung als `1/1` je Zeile – rot, sobald das Soll fehlt
- **Monatsübersicht**: eine Zeile je Person, eine Spalte je Tag. Jeder Tag ist
  geteilt – oben die Arbeitsschicht, unten die Rufbereitschaft, weil beides am
  selben Tag nebeneinander läuft. Ein Klick auf das obere Feld trägt die
  gewählte Arbeitsschicht ein oder aus, ein Klick auf den unteren Streifen die
  Rufbereitschaft. **Shift+Klick** zieht vom zuletzt geklickten Tag bis hierher
  auf – nur innerhalb derselben Zeile und desselben Bandes, also eine ganze
  Woche Rufbereitschaft in zwei Klicks, ohne je eine fremde Zeile zu treffen.
  Ein belegtes Feld lässt sich auf einen anderen Tag **ziehen**, ebenfalls nur
  in derselben Zeile. Die Zeilen lassen sich **sortieren** (Name, Team oder
  „meiste Spätschichten“), und statt des ganzen Monats lässt sich eine
  **einzelne Woche** zeigen – Montag bis Sonntag, auch über den Monatsrand
  hinweg. Beides wird gemerkt. Rechts die Summen F/N/S/R je Person, unten die
  Besetzung je Tag – ebenfalls getrennt, damit eine fehlende Rufbereitschaft
  nicht in der Summe der Arbeitsschichten untergeht
- **Feiertage** DE (BW) und IN inkl. Brückentagen und eigenen Feiertagen.
  Ein Eintrag am Feiertag des eigenen Teams wird abgefragt, nicht verhindert
- **Konfliktprüfung**: wer schon in einer anderen Schicht steht, wird nur nach
  Rückfrage umgetragen – Rufbereitschaft bleibt dabei erhalten. Wer trotzdem in
  Früh und Spät desselben Tages steht, wird im Kalender und in der
  Monatsübersicht markiert
- **Templates** pro Wochentag und Person – nur die Arbeitsschicht, ohne
  Rufbereitschaft – und über einen frei gewählten Zeitraum anwendbar,
  z. B. Januar bis Dezember in einem Zug. Beim Anwenden wählt man zwischen
  *Nur ergänzen* (trägt ein, was fehlt) und *Angleichen*: dann folgt der Plan
  dem Template auch dort, wo schon etwas steht – wer anders eingeteilt ist,
  gibt die Schicht ab, ein „Frei“ räumt den Tag. Wo das Template nichts sagt,
  bleibt alles stehen
- **Rufbereitschaft reihum**: Personen in eine Reihenfolge bringen, Zeitraum
  wählen, und der KW-Plan wird Woche für Woche durchrotiert
- **Rufbereitschaft** in einer Tabelle: je Kalenderwoche der geplante Name und
  daneben, wer tatsächlich im Kalender steht. Noch nicht übertragene Wochen und
  Abweichungen sind markiert und lassen sich einzeln herausfiltern. Eine Woche
  lässt sich zu ihren sieben Tagen aufklappen – dort trägt man eine Vertretung
  für einen einzelnen Tag ein, ohne den Wochenplan zu ändern
- **Massenanlage** für Mitarbeiter und eigene Feiertage: Liste einfügen
  (auch aus Excel), Vorschau prüfen, anlegen
- **Wochenenden** brauchen nur Rufbereitschaft – Früh und Spät werden dort
  nicht als fehlend angemahnt
- **Eine Leiste**: Ansichten, Monat und der Hinweis auf Unterbesetzung stehen
  nebeneinander im Kopf. Ansichten ohne Monatsbezug grauen die Datumssteuerung
  nur aus, statt sie verschwinden zu lassen; alles Seltene – ICS, Drucken,
  Sicherung, Ordner, Anzeigegröße, Theme – liegt im Menü ⋯ rechts
- **Ein Werkzeug**: Person(en) und Schicht stehen zusammen oben in der
  Seitenleiste. Klick trägt ein oder aus, Shift+Klick zieht die gleiche
  Absicht über einen Zeitraum, Strg+Klick sammelt einzelne Tage. Tasten 1–4
  wählen die Schicht, ← → blättern den Monat, T springt auf heute
- **Ein Tagesmenü**: Notiz, Kopieren, Einfügen und Leeren stecken hinter ⋯ in
  der Tageskarte oder hinter dem Rechtsklick
- **Druckansicht** im Querformat: nur die offene Ansicht, mit Kopfzeile,
  ausgeschriebenen Notizen und derselben Legende wie im Fenster – unabhängig
  vom Theme immer auf Weiß
- **Import/Export** als ICS (Kalender) und JSON (vollständiges Backup)
- **Anzeigegröße** über A− / A+ im Menü ⋯ (90 % bis 160 %), die
  Mitarbeiterleiste lässt sich ausklappen – beides wird gemerkt
- **Datenordner frei wählbar**

## Tests

```
go test ./...       # API, Speicher, Feiertage, ICS, Nebenläufigkeit
npm install         # einmalig, holt jsdom und TypeScript
npm test            # Typprüfung, dann Planungsregeln und Oberfläche
npm run typen       # nur die Typprüfung
npm run coverage    # Tests mit Deckungsgrad
```

Die Planungsregeln stehen in `frontend/index.html` im Block
`<script id="regelkern">` – ohne DOM, ohne globalen Zustand, jede Funktion
bekommt ihre Eingaben als Argument. `tests/lade-regelkern.mjs` schneidet den
Block heraus und macht ihn einzeln prüfbar; `tests/lade-oberflaeche.mjs`
startet stattdessen die ganze Seite in jsdom und hängt eine erfundene API
davor, sodass Klickwege wie im Fenster laufen. Die Oberfläche bleibt dabei
eine einzige Datei ohne Laufzeitabhängigkeiten – jsdom ist reine
Entwicklungsausstattung und landet nicht in der EXE.

### Typprüfung

Die Oberfläche bleibt JavaScript ohne Build-Schritt – die EXE bettet genau
`frontend/index.html` ein. Geprüft wird trotzdem mit dem TypeScript-Compiler:
die Typen stehen als JSDoc-Kommentare im Code, `tests/typpruefung.mjs`
schneidet beide `<script>`-Blöcke zeilengenau heraus und lässt `tsc` darüber
laufen, ohne etwas zu erzeugen. Jede Meldung zeigt direkt auf eine Zeile in
`index.html`. TypeScript ist wie jsdom reine Entwicklungsausstattung.

| Teil | Strenge | Warum |
|---|---|---|
| Regelkern | `strict`, nur ES-Bibliothek | Hier stehen die Regeln; ihre Typen tragen bis in jeden Aufruf. Ohne DOM-Bibliothek ist „der Regelkern greift nicht ins Fenster“ eine Regel, die tsc durchsetzt. |
| Oberfläche | `strict` ohne `noImplicitAny` | Der eine fehlende Schalter verlangt einen Typ an jedem Parameter (rund 480 Stellen) und ist ein eigener Durchgang. `strictNullChecks` kam als zweite Stufe dazu. |

Die Einstellungen stehen mit Begründung in `tsconfig.json` und
`tsconfig.regelkern.json`. Das DOM liefert allgemeine Typen – `getElementById`
kennt kein `.value` –, deshalb gibt es oben in der Oberfläche eine Handvoll
Griffe (`dom.eingabe(id)`, `dom.auswahl(id)`, …), die tsc sagen, was an der
Stelle steht. `tests/typpruefung.test.mjs` gleicht jeden davon gegen das Markup
ab: ein Griff, der ein `<input>` verspricht, wo ein `<select>` steht, fällt
dort auf. `@ts-ignore` und Casts auf `any` gibt es nicht; auch das prüft der Test.

Mit `strictNullChecks` kommen `dom.muss(id)` und `dom.einsMuss(wurzel, wahl)`
dazu: sie sprechen aus, was der Code ohnehin annimmt – das Element steht fest
im Markup. Fehlt es doch, werfen sie sofort mit dem Namen im Text, statt drei
Zeilen später an einer Eigenschaft von `null`.

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
| `shifts` | je Zeile ein Eintrag `(Datum, Schicht, Name)` – nur `frueh`, `spaet`, `rufbereitschaft` werden gelesen |
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
go test ./...  # Regeln, API, Speicher, Nebenläufigkeit
npm install    # einmalig, holt jsdom
npm test       # Oberfläche
```

Details zum Build siehe [BUILD.md](BUILD.md).

## Aufbau

```
main.go                  Verdrahtung: Oberfläche einbetten, Server bauen, Fenster öffnen
wails.json               Build-Konfiguration
frontend/index.html      gesamte Oberfläche (HTML/CSS/JS)
internal/
  domain/                Begriffe und Regeln - ohne Datenbank, ohne HTTP
    model.go             Mitarbeiter, Tag, Soll, Template, Änderungseintrag
    slot.go              Operationen auf einem Tag
    holiday.go           Feiertage DE (BW) + IN, Brückentage
    week.go              ISO-Kalenderwochen
    plan.go              was ein Klick, ein Template, ein KW-Plan bedeutet
  store/store.go         SQLite: Schema, Transaktionen, Änderungsverlauf
  httpapi/               Anfragen entgegennehmen, Antworten schreiben
    server.go            Router, Datenordner
    json.go              JSON hinein und hinaus, Fehler melden
    employees.go         Mitarbeiter und Gesamtabzug
    shifts.go            Schichten, Soll, Notizen, Tage einfügen
    holidays.go          Feiertage
    templates.go         Templates und Autoplan
    rufkw.go             Wochenplan der Rufbereitschaft
    system.go            Rückgängig-Sprung, Verlauf, Datenordner, Sicherung
    ics.go               ICS-Export und -Import
  config/config.go       merkt den zuletzt benutzten Datenordner
tests/                   Tests der Oberfläche (Node, siehe oben)
```

**Wie die Abhängigkeiten laufen.** `domain` kennt niemanden. `store` und
`httpapi` kennen `domain`. `main` kennt alle drei. Nichts zeigt zurück – wer
eine Regel ändern will, ändert sie in `domain` und muss weder Handler noch
Datenbank anfassen.

**Warum `main.go` im Wurzelverzeichnis liegt und nicht unter `/cmd`.**
`wails build` übersetzt das Modul im aktuellen Verzeichnis und erwartet den
Ordner aus `wails.json` (`frontend/`) daneben. Ein Hauptpaket unter
`cmd/schichtplaner/` würde bedeuten, entweder das Frontend dorthin zu
verschieben (dann findet `wails.json` es nicht mehr) oder den Build von Hand
nachzubauen. Für **ein** Binary bringt `/cmd` ohnehin nur Ordnung, wo mehrere
liegen – der Gewinn wiegt den Bruch mit dem Build-Werkzeug nicht auf.

**Kein `/pkg`.** Nichts in diesem Projekt ist dafür gedacht, von außen benutzt
zu werden. `internal/` sagt das dem Compiler; `/pkg` würde das Gegenteil
behaupten.

**Wails steht nur in `main.go`.** Der Ordnerdialog kommt als Funktion in den
Server hinein (`httpapi.New(seite, dialog)`). Deshalb läuft der ganze Kern in
Tests ohne Fenster – und deshalb prüft `go test ./...` echte Handler statt
Attrappen.

Das Frontend spricht das Backend ausschließlich über `/api/…` an; die Routen
stehen in [internal/httpapi/server.go](internal/httpapi/server.go).
`GET /api/history?limit=200` liefert das Änderungsprotokoll.

## Feiertage

Gesetzliche Feiertage DE (Baden-Württemberg) werden gerechnet, die indischen
Fixtermine ebenso. **Holi und Diwali** folgen dem lunisolaren Kalender und
haben keine Formel – sie stehen als Tabelle in [internal/domain/holiday.go](internal/domain/holiday.go) und
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
