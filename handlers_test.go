package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestApp returns an App writing to a throwaway data folder.
func newTestApp(t *testing.T) *App {
	t.Helper()
	return &App{dataFolder: t.TempDir()}
}

// call runs one request through the router and decodes the JSON response.
func call(t *testing.T, a *App, method, path, body string) map[string]interface{} {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("%s %s: status %d, body %s", method, path, w.Code, w.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: bad JSON %q: %v", method, path, w.Body.String(), err)
	}
	return out
}

func TestRufKWRoundTrip(t *testing.T) {
	a := newTestApp(t)

	// The frontend posts the plan wrapped in a "ruf_kw" envelope.
	call(t, a, http.MethodPost, "/api/ruf_kw", `{"ruf_kw":{"2026-W02":["Anna"]}}`)

	got := call(t, a, http.MethodGet, "/api/data", "")
	plan, ok := got["ruf_kw"].(map[string]interface{})
	if !ok {
		t.Fatalf("ruf_kw missing or wrong type: %#v", got["ruf_kw"])
	}
	if _, ok := plan["2026-W02"]; !ok {
		t.Fatalf("KW key lost, plan is %#v", plan)
	}

	// Applying the plan must reach the individual days of that week.
	res := call(t, a, http.MethodPost, "/api/ruf_kw/apply", `{"year":2026,"month":1,"overwrite":false}`)
	if res["applied"].(float64) != 7 {
		t.Fatalf("expected 7 applied days, got %v", res["applied"])
	}
}

func TestDeleteAndRestoreEmployee(t *testing.T) {
	a := newTestApp(t)

	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Anna","team":"DE","color":"#fff"}`)
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-01"],"schicht":"frueh","name":"Anna","action":"add"}`)

	del := call(t, a, http.MethodDelete, "/api/mitarbeiter/Anna", "")
	backup, ok := del["backup"].(map[string]interface{})
	if !ok || backup["2026-04-01"] == nil {
		t.Fatalf("delete did not return a usable backup: %#v", del["backup"])
	}

	// The shift entry must be gone from the stored data, not just from the UI.
	data := call(t, a, http.MethodGet, "/api/data", "")
	day := data["schichten"].(map[string]interface{})["2026-04-01"].(map[string]interface{})
	if len(day["frueh"].([]interface{})) != 0 {
		t.Fatalf("deleted employee still in shift: %#v", day["frueh"])
	}

	// Re-adding plus restore brings the entry back and keeps the employee list.
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Anna","team":"DE"}`)
	call(t, a, http.MethodPost, "/api/mitarbeiter/restore",
		`{"name":"Anna","entries":{"2026-04-01":{"frueh":true}}}`)

	data = call(t, a, http.MethodGet, "/api/data", "")
	if n := len(data["mitarbeiter"].([]interface{})); n != 1 {
		t.Fatalf("restore clobbered the employee list, %d left", n)
	}
	day = data["schichten"].(map[string]interface{})["2026-04-01"].(map[string]interface{})
	if len(day["frueh"].([]interface{})) != 1 {
		t.Fatalf("entry not restored: %#v", day["frueh"])
	}
}

func TestRenameEmployeeUpdatesReferences(t *testing.T) {
	a := newTestApp(t)

	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Anna","team":"DE"}`)
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-01"],"schicht":"frueh","name":"Anna","action":"add"}`)
	call(t, a, http.MethodPost, "/api/templates", `{"name":"Standard","template":{"Anna":{"0":"frueh"}}}`)
	call(t, a, http.MethodPost, "/api/ruf_kw", `{"ruf_kw":{"2026-W02":["Anna"]}}`)

	call(t, a, http.MethodPut, "/api/mitarbeiter/Anna", `{"name":"Anna Neu","team":"DE"}`)

	data := call(t, a, http.MethodGet, "/api/data", "")
	day := data["schichten"].(map[string]interface{})["2026-04-01"].(map[string]interface{})
	if day["frueh"].([]interface{})[0] != "Anna Neu" {
		t.Errorf("shift not renamed: %#v", day["frueh"])
	}
	tmpl := data["templates"].(map[string]interface{})["Standard"].(map[string]interface{})
	if _, ok := tmpl["Anna Neu"]; !ok {
		t.Errorf("template not renamed: %#v", tmpl)
	}
	kw := data["ruf_kw"].(map[string]interface{})["2026-W02"].([]interface{})
	if kw[0] != "Anna Neu" {
		t.Errorf("KW plan not renamed: %#v", kw)
	}
}

func TestSchichtIgnoresMalformedDates(t *testing.T) {
	a := newTestApp(t)
	// A short date must not panic the handler.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["","2026-04-01"],"schicht":"frueh","name":"Anna","action":"add"}`)
}

func TestICSRoundTrip(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-01"],"schicht":"spaet","name":"Anna","action":"add"}`)

	r := httptest.NewRequest(http.MethodGet, "/api/export_ics?year=2026&month=4", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	ics := w.Body.String()
	if !strings.Contains(ics, "SUMMARY:Anna – Spätschicht") {
		t.Fatalf("event missing from export:\n%s", ics)
	}
}

func TestToggleUsesTheSameConflictRules(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Anna","team":"DE"}`)

	// 1. Mai 2026 is a public holiday in DE, so a toggle must ask first -
	// exactly like an add does.
	res := call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"toggle"}`)
	day := res["results"].(map[string]interface{})["2026-05-01"].(map[string]interface{})
	if day["error"] != "holiday_conflict" {
		t.Fatalf("toggle skipped the holiday check: %#v", day)
	}

	// 2. Forced toggle enters the shift and reports the warning.
	res = call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"toggle","force":true}`)
	if len(res["hol_warnings"].([]interface{})) != 1 {
		t.Fatalf("expected a holiday warning, got %#v", res["hol_warnings"])
	}

	// 3. Toggling again removes it.
	res = call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-05-01"],"schicht":"frueh","name":"Anna","action":"toggle","force":true}`)
	day = res["results"].(map[string]interface{})["2026-05-01"].(map[string]interface{})
	if len(day["frueh"].([]interface{})) != 0 {
		t.Fatalf("second toggle did not remove the entry: %#v", day["frueh"])
	}
}

func TestNeedsConfirmBeforeReplacingAWorkShift(t *testing.T) {
	a := newTestApp(t)
	call(t, a, http.MethodPost, "/api/mitarbeiter", `{"name":"Anna","team":"DE"}`)
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"frueh","name":"Anna","action":"add"}`)

	res := call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"spaet","name":"Anna","action":"add"}`)
	day := res["results"].(map[string]interface{})["2026-04-02"].(map[string]interface{})
	if day["error"] != "needs_confirm" {
		t.Fatalf("expected needs_confirm, got %#v", day)
	}

	// Rufbereitschaft is never reported as the blocking shift: it may run
	// alongside a work shift, so replacing one work shift by another leaves it be.
	call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"rufbereitschaft","name":"Anna","action":"add","force":true}`)
	res = call(t, a, http.MethodPost, "/api/schicht",
		`{"dates":["2026-04-02"],"schicht":"spaet","name":"Anna","action":"add"}`)
	day = res["results"].(map[string]interface{})["2026-04-02"].(map[string]interface{})
	blocking := day["blocking"].([]interface{})
	if len(blocking) != 1 || blocking[0] != "frueh" {
		t.Fatalf("expected only frueh to block, got %#v", blocking)
	}
}
