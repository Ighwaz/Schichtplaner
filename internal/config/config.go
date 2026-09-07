// Package config merkt sich zwischen zwei Starts, welcher Datenordner zuletzt
// in Gebrauch war. Mehr steht nicht in der Datei, und mehr soll auch nicht
// hinein: der Plan selbst gehört in den Datenordner, nicht ins Benutzerprofil.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config ist der Inhalt der Konfigurationsdatei.
type Config struct {
	DataFolder string `json:"data_folder"`
}

// Path nennt den Ort der Konfigurationsdatei. Als Variable, damit Tests sie in
// ein Wegwerfverzeichnis umlenken können - sonst schriebe jeder Test über den
// Ordnerwechsel in das echte Benutzerprofil.
var Path = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".schichtplaner_config.json")
}

// Load liest die Konfiguration. Fehlt die Datei oder ist sie unlesbar, gilt
// die leere Konfiguration - beim ersten Start ist das der Normalfall, und ein
// kaputter Merkzettel darf den Start nicht verhindern.
func Load() Config {
	raw, err := os.ReadFile(Path())
	if err != nil {
		return Config{}
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}
	}
	return cfg
}

// Save schreibt die Konfiguration.
func Save(cfg Config) error {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("Konfiguration nicht darstellbar: %w", err)
	}
	if err := os.WriteFile(Path(), raw, 0o644); err != nil {
		return fmt.Errorf("Konfiguration %s schreiben: %w", Path(), err)
	}
	return nil
}
