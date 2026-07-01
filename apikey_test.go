package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAPIKeys(t *testing.T) {
	want := []HeadscaleAPIKey{
		{ID: "1", Prefix: "hskey-api-abc", Expiration: "2027-01-01T00:00:00Z", CreatedAt: "2026-01-01T00:00:00Z"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/apikey" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string][]HeadscaleAPIKey{"apiKeys": want})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := listAPIKeys(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Prefix != "hskey-api-abc" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestCreateAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/apikey" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{"apiKey": "hskey-api-newkey123"})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := createAPIKey(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hskey-api-newkey123" {
		t.Errorf("got key %q, want %q", got, "hskey-api-newkey123")
	}
}

func TestExpireAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/apikey/hskey-api-abc" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	if err := expireAPIKey(cfg, "hskey-api-abc"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
