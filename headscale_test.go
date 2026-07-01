package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindNodeByName_MatchesGivenName(t *testing.T) {
	nodes := []HeadscaleNode{
		{ID: "1", Name: "laptop", GivenName: "alice-laptop"},
		{ID: "2", Name: "server", GivenName: ""},
	}
	n, err := findNodeByName(nodes, "alice-laptop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.ID != "1" {
		t.Errorf("got ID %q, want %q", n.ID, "1")
	}
}

func TestFindNodeByName_MatchesHostname(t *testing.T) {
	nodes := []HeadscaleNode{
		{ID: "2", Name: "server", GivenName: ""},
	}
	n, err := findNodeByName(nodes, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.ID != "2" {
		t.Errorf("got ID %q, want %q", n.ID, "2")
	}
}

func TestFindNodeByName_NotFound(t *testing.T) {
	nodes := []HeadscaleNode{{ID: "1", Name: "laptop", GivenName: "alice-laptop"}}
	_, err := findNodeByName(nodes, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRenameNode(t *testing.T) {
	want := HeadscaleNode{ID: "1", Name: "laptop", GivenName: "new-name"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("got method %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/node/1/rename/new-name" {
			t.Errorf("got path %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]HeadscaleNode{"node": want})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test-key"}
	got, err := renameNode(cfg, "1", "new-name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GivenName != want.GivenName {
		t.Errorf("got GivenName %q, want %q", got.GivenName, want.GivenName)
	}
}
