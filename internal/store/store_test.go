package store

import (
	"os"
	"path/filepath"
	"testing"
)

// Der Plan älterer Fassungen lag als JSON-Datei im Datenordner. Beim ersten
// Oeffnen wandert er in die Datenbank - danach nie wieder, sonst überschriebe
// ein Neustart neuere Aenderungen mit dem alten Stand.
func TestAlterPlanWirdGenauEinmalUebernommen(t *testing.T) {
	folder := t.TempDir()
	legacy := `{"mitarbeiter":[{"name":"Alt","team":"DE","color":"#fff","prefs":{}}],
		"schichten":{"2026-03-02":{"frueh":["Alt"]}},"notizen":{"2026-03-02":"Notiz"},
		"soll":{"frueh":2,"spaet":1,"rufbereitschaft":1},"templates":{},"ruf_kw":{}}`
	if err := os.WriteFile(filepath.Join(folder, legacyFileName), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Open(t.Context(), folder)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	d, err := s.Load(t.Context())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(d.Mitarbeiter) != 1 || d.Mitarbeiter[0].Name != "Alt" {
		t.Fatalf("employee not imported: %#v", d.Mitarbeiter)
	}
	if d.Notizen["2026-03-02"] != "Notiz" || d.Soll.Frueh != 2 {
		t.Fatalf("notes/soll not imported: %#v %#v", d.Notizen, d.Soll)
	}

	// Ein zweiter Start darf die JSON-Datei nicht erneut über neuere
	// Änderungen legen.
	if _, _, err := s.DeleteEmployee(t.Context(), "Alt"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s2, err := Open(t.Context(), folder)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	d, err = s2.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Mitarbeiter) != 0 {
		t.Fatalf("legacy JSON was imported a second time: %#v", d.Mitarbeiter)
	}
}
