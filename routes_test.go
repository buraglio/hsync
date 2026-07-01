package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListRoutes(t *testing.T) {
	want := []HeadscaleRoute{
		{ID: "1", Prefix: "10.0.0.0/24", Advertised: true, Enabled: false,
			Node: HeadscaleNode{ID: "5", Name: "gateway", GivenName: "gw"}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/routes" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string][]HeadscaleRoute{"routes": want})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := listRoutes(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Prefix != "10.0.0.0/24" {
		t.Errorf("unexpected routes: %v", got)
	}
}

func TestEnableRoute(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("got method %q, want POST", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]HeadscaleRoute{"route": {ID: "1", Enabled: true}})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	if err := enableRoute(cfg, "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/v1/routes/1/enable" {
		t.Errorf("got path %q, want /api/v1/routes/1/enable", gotPath)
	}
}

func TestDisableRoute(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]HeadscaleRoute{"route": {ID: "1", Enabled: false}})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	if err := disableRoute(cfg, "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/v1/routes/1/disable" {
		t.Errorf("got path %q, want /api/v1/routes/1/disable", gotPath)
	}
}

func TestDeleteRoute(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	if err := deleteRoute(cfg, "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("got method %q, want DELETE", gotMethod)
	}
}
