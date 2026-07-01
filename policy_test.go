package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPolicy(t *testing.T) {
	want := HeadscalePolicy{
		Policy:    `{"hosts":{}}`,
		UpdatedAt: "2026-01-01T00:00:00Z",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/v1/policy" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := getPolicy(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Policy != `{"hosts":{}}` {
		t.Errorf("unexpected Policy: %q", got.Policy)
	}
}

func TestSetPolicy(t *testing.T) {
	newPolicy := `{"hosts":{"example":"100.64.0.1"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/v1/policy" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["policy"] != newPolicy {
			t.Errorf("unexpected body policy: %q", body["policy"])
		}
		json.NewEncoder(w).Encode(HeadscalePolicy{Policy: newPolicy, UpdatedAt: "2026-01-01T00:00:00Z"})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := setPolicy(cfg, newPolicy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Policy != newPolicy {
		t.Errorf("unexpected Policy: %q", got.Policy)
	}
}
