package domain

import (
	"fmt"
	"time"
)

// KWNames reads a KW entry, which is either a single name or a list of names.
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

// ISOWeekKey formats a date as the KW key used by the plan, e.g. 2026-W12.
func ISOWeekKey(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}
