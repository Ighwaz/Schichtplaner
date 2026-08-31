package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const dbFileName = "schichtplan.db"

// Store keeps the shift plan in a SQLite database inside the data folder.
// Every write runs in a transaction and touches only the affected rows, and
// every change is appended to the changelog table.
type Store struct {
	db     *sql.DB
	folder string
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS employees (
	name  TEXT PRIMARY KEY,
	team  TEXT NOT NULL DEFAULT '',
	color TEXT NOT NULL DEFAULT '',
	icon  TEXT NOT NULL DEFAULT '',
	prefs TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS shifts (
	date  TEXT NOT NULL,
	shift TEXT NOT NULL,
	name  TEXT NOT NULL,
	PRIMARY KEY (date, shift, name)
);
CREATE INDEX IF NOT EXISTS shifts_by_date ON shifts(date);
CREATE INDEX IF NOT EXISTS shifts_by_name ON shifts(name);
CREATE TABLE IF NOT EXISTS notes (
	date TEXT PRIMARY KEY,
	text TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS custom_holidays (
	date    TEXT NOT NULL,
	name    TEXT NOT NULL,
	country TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (date, name)
);
CREATE TABLE IF NOT EXISTS templates (
	name TEXT PRIMARY KEY,
	data TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS ruf_kw (
	kw    TEXT PRIMARY KEY,
	names TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS changelog (
	id     INTEGER PRIMARY KEY AUTOINCREMENT,
	ts     TEXT NOT NULL,
	action TEXT NOT NULL,
	detail TEXT NOT NULL DEFAULT ''
);
`

// ── Open / close ──────────────────────────────────────────────────────────────

func openStore(folder string) (*Store, error) {
	path := filepath.Join(folder, dbFileName)
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection is enough and keeps writers from tripping over each other.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("Datenbank %s: %w", path, err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("Schema anlegen: %w", err)
	}
	s := &Store{db: db, folder: folder}
	if err := s.importLegacyJSON(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// importLegacyJSON moves an existing schichtplan_daten.json into the database,
// but only into an empty one - an existing database is never overwritten.
func (s *Store) importLegacyJSON() error {
	var setting string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key='json_imported'`).Scan(&setting)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	var rows int
	if err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM employees) + (SELECT COUNT(*) FROM shifts)`).Scan(&rows); err != nil {
		return err
	}
	if rows > 0 {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(s.folder, dataFileName))
	if err != nil {
		return nil // no legacy file, nothing to do
	}
	var d AppData
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("%s ist kein gültiges JSON: %w", dataFileName, err)
	}
	normalizeData(&d)
	if err := s.ReplaceAll(d, "import:json"); err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO settings (key, value) VALUES ('json_imported', ?)`, timestamp())
	return err
}

func timestamp() string { return time.Now().Format(time.RFC3339) }

// plural formats a count with the fitting German word, e.g. "1 Eintrag".
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// ── Reading ───────────────────────────────────────────────────────────────────

// Load assembles the whole plan in the shape the frontend expects.
func (s *Store) Load() (AppData, error) {
	d := defaultData()

	rows, err := s.db.Query(`SELECT name, team, color, icon, prefs FROM employees ORDER BY rowid`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var m Employee
		var prefs string
		if err := rows.Scan(&m.Name, &m.Team, &m.Color, &m.Icon, &prefs); err != nil {
			rows.Close()
			return d, err
		}
		m.Prefs = map[string]string{}
		json.Unmarshal([]byte(prefs), &m.Prefs)
		d.Mitarbeiter = append(d.Mitarbeiter, m)
	}
	if err := closeRows(rows); err != nil {
		return d, err
	}

	rows, err = s.db.Query(`SELECT date, shift, name FROM shifts ORDER BY rowid`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var date, shift, name string
		if err := rows.Scan(&date, &shift, &name); err != nil {
			rows.Close()
			return d, err
		}
		slot := slotFor(&d, date)
		addToSlot(&slot, shift, name)
		d.Schichten[date] = slot
	}
	if err := closeRows(rows); err != nil {
		return d, err
	}

	rows, err = s.db.Query(`SELECT date, text FROM notes`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var date, text string
		if err := rows.Scan(&date, &text); err != nil {
			rows.Close()
			return d, err
		}
		d.Notizen[date] = text
	}
	if err := closeRows(rows); err != nil {
		return d, err
	}

	if d.CustomHolidays, err = s.CustomHolidays(); err != nil {
		return d, err
	}

	rows, err = s.db.Query(`SELECT name, data FROM templates ORDER BY name`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var name, data string
		if err := rows.Scan(&name, &data); err != nil {
			rows.Close()
			return d, err
		}
		t := Template{}
		json.Unmarshal([]byte(data), &t)
		d.Templates[name] = t
	}
	if err := closeRows(rows); err != nil {
		return d, err
	}

	if d.RufKW, err = s.RufKW(); err != nil {
		return d, err
	}

	var soll string
	switch err := s.db.QueryRow(`SELECT value FROM settings WHERE key='soll'`).Scan(&soll); err {
	case nil:
		json.Unmarshal([]byte(soll), &d.Soll)
	case sql.ErrNoRows:
	default:
		return d, err
	}
	normalizeData(&d)
	return d, nil
}

func closeRows(rows *sql.Rows) error {
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	return rows.Close()
}

// Day returns the entries of a single date.
func (s *Store) Day(date string) (DaySlot, error) {
	slot := emptySlot()
	rows, err := s.db.Query(`SELECT shift, name FROM shifts WHERE date = ? ORDER BY rowid`, date)
	if err != nil {
		return slot, err
	}
	for rows.Next() {
		var shift, name string
		if err := rows.Scan(&shift, &name); err != nil {
			rows.Close()
			return slot, err
		}
		addToSlot(&slot, shift, name)
	}
	return slot, closeRows(rows)
}

// Team returns the team of an employee, or "" if the name is unknown.
func (s *Store) Team(name string) (string, error) {
	var team string
	err := s.db.QueryRow(`SELECT team FROM employees WHERE name = ?`, name).Scan(&team)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return team, err
}

func (s *Store) Employees() ([]Employee, error) {
	d, err := s.Load()
	return d.Mitarbeiter, err
}

func (s *Store) CustomHolidays() ([]CustomHoliday, error) {
	out := []CustomHoliday{}
	rows, err := s.db.Query(`SELECT date, name, country FROM custom_holidays ORDER BY date, name`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var ch CustomHoliday
		if err := rows.Scan(&ch.Date, &ch.Name, &ch.Country); err != nil {
			rows.Close()
			return out, err
		}
		out = append(out, ch)
	}
	return out, closeRows(rows)
}

func (s *Store) RufKW() (map[string]interface{}, error) {
	out := map[string]interface{}{}
	rows, err := s.db.Query(`SELECT kw, names FROM ruf_kw ORDER BY kw`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var kw, names string
		if err := rows.Scan(&kw, &names); err != nil {
			rows.Close()
			return out, err
		}
		var v interface{}
		if json.Unmarshal([]byte(names), &v) == nil {
			out[kw] = v
		}
	}
	return out, closeRows(rows)
}

// ── Writing ───────────────────────────────────────────────────────────────────

// tx runs fn in a transaction and appends one changelog entry for it.
func (s *Store) tx(action, detail string, fn func(*sql.Tx) error) error {
	t, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(t); err != nil {
		t.Rollback()
		return err
	}
	if action != "" {
		if _, err := t.Exec(`INSERT INTO changelog (ts, action, detail) VALUES (?, ?, ?)`,
			timestamp(), action, detail); err != nil {
			t.Rollback()
			return err
		}
	}
	return t.Commit()
}

// ReplaceAll overwrites the entire database content - used by import, snapshot
// and the one-time migration of the old JSON file.
func (s *Store) ReplaceAll(d AppData, action string) error {
	detail := plural(len(d.Mitarbeiter), "Mitarbeiter", "Mitarbeiter") + ", " + plural(len(d.Schichten), "Tag", "Tage")
	return s.tx(action, detail, func(t *sql.Tx) error {
		for _, table := range []string{"employees", "shifts", "notes", "custom_holidays", "templates", "ruf_kw"} {
			if _, err := t.Exec("DELETE FROM " + table); err != nil {
				return err
			}
		}
		for _, m := range d.Mitarbeiter {
			if err := insertEmployee(t, m); err != nil {
				return err
			}
		}
		for date, slot := range d.Schichten {
			if err := insertDay(t, date, slot); err != nil {
				return err
			}
		}
		for date, text := range d.Notizen {
			if _, err := t.Exec(`INSERT INTO notes (date, text) VALUES (?, ?)`, date, text); err != nil {
				return err
			}
		}
		for _, ch := range d.CustomHolidays {
			if _, err := t.Exec(`INSERT OR REPLACE INTO custom_holidays (date, name, country) VALUES (?, ?, ?)`,
				ch.Date, ch.Name, ch.Country); err != nil {
				return err
			}
		}
		for name, tmpl := range d.Templates {
			if err := insertTemplate(t, name, tmpl); err != nil {
				return err
			}
		}
		for kw, names := range d.RufKW {
			if err := insertRufKW(t, kw, names); err != nil {
				return err
			}
		}
		return setSetting(t, "soll", d.Soll)
	})
}

func insertEmployee(t *sql.Tx, m Employee) error {
	prefs, err := json.Marshal(m.Prefs)
	if err != nil {
		return err
	}
	_, err = t.Exec(`INSERT OR REPLACE INTO employees (name, team, color, icon, prefs) VALUES (?, ?, ?, ?, ?)`,
		m.Name, m.Team, m.Color, m.Icon, string(prefs))
	return err
}

// insertDay writes the entries of one day; the caller has cleared it before.
func insertDay(t *sql.Tx, date string, slot DaySlot) error {
	var err error
	forEachShift(&slot, func(shift string, names *[]string) {
		for _, name := range *names {
			if err != nil {
				return
			}
			_, err = t.Exec(`INSERT OR IGNORE INTO shifts (date, shift, name) VALUES (?, ?, ?)`, date, shift, name)
		}
	})
	return err
}

func insertTemplate(t *sql.Tx, name string, tmpl Template) error {
	data, err := json.Marshal(tmpl)
	if err != nil {
		return err
	}
	_, err = t.Exec(`INSERT OR REPLACE INTO templates (name, data) VALUES (?, ?)`, name, string(data))
	return err
}

func insertRufKW(t *sql.Tx, kw string, names interface{}) error {
	raw, err := json.Marshal(names)
	if err != nil {
		return err
	}
	_, err = t.Exec(`INSERT OR REPLACE INTO ruf_kw (kw, names) VALUES (?, ?)`, kw, string(raw))
	return err
}

func setSetting(t *sql.Tx, key string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = t.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, key, string(raw))
	return err
}

// ── Shifts ────────────────────────────────────────────────────────────────────

// ShiftChange is one entry to add or remove.
type ShiftChange struct {
	Date  string
	Shift string
	Name  string
}

// AddShifts inserts entries and returns how many were actually new.
func (s *Store) AddShifts(action string, changes []ShiftChange) (int, error) {
	added := 0
	err := s.tx(action, plural(len(changes), "Eintrag", "Einträge"), func(t *sql.Tx) error {
		for _, c := range changes {
			if slotField(&DaySlot{}, c.Shift) == nil {
				continue
			}
			res, err := t.Exec(`INSERT OR IGNORE INTO shifts (date, shift, name) VALUES (?, ?, ?)`,
				c.Date, c.Shift, c.Name)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				added++
			}
		}
		return nil
	})
	return added, err
}

// RemoveShifts deletes entries and returns how many were present.
func (s *Store) RemoveShifts(action string, changes []ShiftChange) (int, error) {
	removed := 0
	err := s.tx(action, plural(len(changes), "Eintrag", "Einträge"), func(t *sql.Tx) error {
		for _, c := range changes {
			res, err := t.Exec(`DELETE FROM shifts WHERE date = ? AND shift = ? AND name = ?`,
				c.Date, c.Shift, c.Name)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				removed++
			}
		}
		return nil
	})
	return removed, err
}

// ReplaceDays overwrites the given dates with slot.
func (s *Store) ReplaceDays(dates []string, slot DaySlot) error {
	return s.tx("tag:ersetzen", plural(len(dates), "Tag", "Tage"), func(t *sql.Tx) error {
		for _, date := range dates {
			if _, err := t.Exec(`DELETE FROM shifts WHERE date = ?`, date); err != nil {
				return err
			}
			if err := insertDay(t, date, slot); err != nil {
				return err
			}
		}
		return nil
	})
}

// MergeDays adds the entries of slot to the given dates.
func (s *Store) MergeDays(dates []string, slot DaySlot) error {
	return s.tx("tag:einfügen", plural(len(dates), "Tag", "Tage"), func(t *sql.Tx) error {
		for _, date := range dates {
			if err := insertDay(t, date, slot); err != nil {
				return err
			}
		}
		return nil
	})
}

// ReplaceAllShifts swaps the complete shift table - used by undo/redo, which
// sends the whole plan back.
func (s *Store) ReplaceAllShifts(days map[string]DaySlot) error {
	return s.tx("snapshot", plural(len(days), "Tag", "Tage"), func(t *sql.Tx) error {
		if _, err := t.Exec(`DELETE FROM shifts`); err != nil {
			return err
		}
		for date, slot := range days {
			if err := insertDay(t, date, slot); err != nil {
				return err
			}
		}
		return nil
	})
}

// ── Employees ─────────────────────────────────────────────────────────────────

func (s *Store) AddEmployee(m Employee) error {
	return s.tx("mitarbeiter:neu", m.Name, func(t *sql.Tx) error {
		return insertEmployee(t, m)
	})
}

func (s *Store) UpdateEmployee(oldName string, m Employee) error {
	return s.tx("mitarbeiter:ändern", oldName+" -> "+m.Name, func(t *sql.Tx) error {
		if _, err := t.Exec(`UPDATE employees SET name = ?, team = ?, icon = ? WHERE name = ?`,
			m.Name, m.Team, m.Icon, oldName); err != nil {
			return err
		}
		if m.Color != "" {
			if _, err := t.Exec(`UPDATE employees SET color = ? WHERE name = ?`, m.Color, m.Name); err != nil {
				return err
			}
		}
		if m.Name == oldName {
			return nil
		}
		if _, err := t.Exec(`UPDATE OR REPLACE shifts SET name = ? WHERE name = ?`, m.Name, oldName); err != nil {
			return err
		}
		return renameInBlobs(t, oldName, m.Name)
	})
}

// renameInBlobs rewrites the name inside templates and the KW plan, which are
// stored as JSON documents.
func renameInBlobs(t *sql.Tx, oldName, newName string) error {
	rows, err := t.Query(`SELECT name, data FROM templates`)
	if err != nil {
		return err
	}
	tmpls := map[string]Template{}
	for rows.Next() {
		var name, data string
		if err := rows.Scan(&name, &data); err != nil {
			rows.Close()
			return err
		}
		tmpl := Template{}
		json.Unmarshal([]byte(data), &tmpl)
		if days, ok := tmpl[oldName]; ok {
			delete(tmpl, oldName)
			tmpl[newName] = days
			tmpls[name] = tmpl
		}
	}
	if err := closeRows(rows); err != nil {
		return err
	}
	for name, tmpl := range tmpls {
		if err := insertTemplate(t, name, tmpl); err != nil {
			return err
		}
	}

	rows, err = t.Query(`SELECT kw, names FROM ruf_kw`)
	if err != nil {
		return err
	}
	plans := map[string]interface{}{}
	for rows.Next() {
		var kw, raw string
		if err := rows.Scan(&kw, &raw); err != nil {
			rows.Close()
			return err
		}
		var v interface{}
		if json.Unmarshal([]byte(raw), &v) != nil {
			continue
		}
		if replaced, changed := replaceName(v, oldName, newName); changed {
			plans[kw] = replaced
		}
	}
	if err := closeRows(rows); err != nil {
		return err
	}
	for kw, v := range plans {
		if err := insertRufKW(t, kw, v); err != nil {
			return err
		}
	}
	return nil
}

// replaceName swaps oldName for newName in a KW entry, which is either a single
// name or a list of names.
func replaceName(v interface{}, oldName, newName string) (interface{}, bool) {
	switch val := v.(type) {
	case string:
		if val == oldName {
			return newName, true
		}
	case []interface{}:
		changed := false
		for i, item := range val {
			if s, ok := item.(string); ok && s == oldName {
				val[i] = newName
				changed = true
			}
		}
		return val, changed
	}
	return v, false
}

// DeleteEmployee removes the employee and every shift entry of that name, and
// returns those entries so they can be restored later.
func (s *Store) DeleteEmployee(name string) (map[string]map[string]bool, error) {
	backup := map[string]map[string]bool{}
	err := s.tx("mitarbeiter:löschen", name, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT date, shift FROM shifts WHERE name = ?`, name)
		if err != nil {
			return err
		}
		for rows.Next() {
			var date, shift string
			if err := rows.Scan(&date, &shift); err != nil {
				rows.Close()
				return err
			}
			if backup[date] == nil {
				backup[date] = map[string]bool{}
			}
			backup[date][shift] = true
		}
		if err := closeRows(rows); err != nil {
			return err
		}
		if _, err := t.Exec(`DELETE FROM shifts WHERE name = ?`, name); err != nil {
			return err
		}
		_, err = t.Exec(`DELETE FROM employees WHERE name = ?`, name)
		return err
	})
	return backup, err
}

func (s *Store) SetColor(name, color string) error {
	return s.tx("mitarbeiter:farbe", name, func(t *sql.Tx) error {
		_, err := t.Exec(`UPDATE employees SET color = ? WHERE name = ?`, color, name)
		return err
	})
}

func (s *Store) SetPrefs(name string, prefs map[string]string) error {
	return s.tx("mitarbeiter:wünsche", name, func(t *sql.Tx) error {
		raw, err := json.Marshal(prefs)
		if err != nil {
			return err
		}
		_, err = t.Exec(`UPDATE employees SET prefs = ? WHERE name = ?`, string(raw), name)
		return err
	})
}

// ── Notes, Soll, holidays, templates, KW plan ─────────────────────────────────

func (s *Store) SetNote(date, text string) error {
	return s.tx("notiz", date, func(t *sql.Tx) error {
		if text == "" {
			_, err := t.Exec(`DELETE FROM notes WHERE date = ?`, date)
			return err
		}
		_, err := t.Exec(`INSERT OR REPLACE INTO notes (date, text) VALUES (?, ?)`, date, text)
		return err
	})
}

func (s *Store) SetSoll(soll SollBesetzung) error {
	return s.tx("soll", "", func(t *sql.Tx) error {
		return setSetting(t, "soll", soll)
	})
}

func (s *Store) AddCustomHoliday(ch CustomHoliday) error {
	return s.tx("feiertag:neu", ch.Date+" "+ch.Name, func(t *sql.Tx) error {
		_, err := t.Exec(`INSERT OR REPLACE INTO custom_holidays (date, name, country) VALUES (?, ?, ?)`,
			ch.Date, ch.Name, ch.Country)
		return err
	})
}

func (s *Store) DeleteCustomHoliday(date, name string) error {
	return s.tx("feiertag:löschen", date+" "+name, func(t *sql.Tx) error {
		_, err := t.Exec(`DELETE FROM custom_holidays WHERE date = ? AND name = ?`, date, name)
		return err
	})
}

func (s *Store) SaveTemplate(name string, tmpl Template) error {
	return s.tx("template:speichern", name, func(t *sql.Tx) error {
		return insertTemplate(t, name, tmpl)
	})
}

func (s *Store) DeleteTemplate(name string) error {
	return s.tx("template:löschen", name, func(t *sql.Tx) error {
		_, err := t.Exec(`DELETE FROM templates WHERE name = ?`, name)
		return err
	})
}

func (s *Store) SaveRufKW(plan map[string]interface{}) error {
	return s.tx("kw-plan:speichern", plural(len(plan), "Woche", "Wochen"), func(t *sql.Tx) error {
		if _, err := t.Exec(`DELETE FROM ruf_kw`); err != nil {
			return err
		}
		for kw, names := range plan {
			if err := insertRufKW(t, kw, names); err != nil {
				return err
			}
		}
		return nil
	})
}

// ── History ───────────────────────────────────────────────────────────────────

// ChangeEntry is one line of the changelog.
type ChangeEntry struct {
	Time   string `json:"time"`
	Action string `json:"action"`
	Detail string `json:"detail"`
}

func (s *Store) History(limit int) ([]ChangeEntry, error) {
	out := []ChangeEntry{}
	rows, err := s.db.Query(`SELECT ts, action, detail FROM changelog ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var e ChangeEntry
		if err := rows.Scan(&e.Time, &e.Action, &e.Detail); err != nil {
			rows.Close()
			return out, err
		}
		out = append(out, e)
	}
	return out, closeRows(rows)
}
