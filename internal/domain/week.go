package domain

import (
	"fmt"
	"time"
)

// KWNames liest einen Wocheneintrag aus. Er hält entweder einen Namen oder
// eine Liste von Namen; alles andere wird übergangen.
func KWNames(raw any) []string {
	switch v := raw.(type) {
	case string:
		return []string{v}
	case []any:
		var names []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				names = append(names, s)
			}
		}
		return names
	}
	return nil
}

// ISOWeekKey bildet den Wochenschlüssel eines Tages, wie ihn der Plan benutzt:
// etwa 2026-W12.
func ISOWeekKey(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}
