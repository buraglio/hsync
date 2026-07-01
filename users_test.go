package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListUsers(t *testing.T) {
	want := []HeadscaleUser{{ID: "1", Name: "alice"}, {ID: "2", Name: "bob"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/user" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string][]HeadscaleUser{"users": want})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := listUsers(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Name != "alice" {
		t.Errorf("unexpected users: %v", got)
	}
}

func TestCreateUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/user" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]HeadscaleUser{"user": {ID: "3", Name: body["name"]}})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := createUser(cfg, "charlie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "charlie" {
		t.Errorf("got name %q, want %q", got.Name, "charlie")
	}
}

func TestDeleteUser(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodDelete {
			t.Errorf("got method %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	if err := deleteUser(cfg, "alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/v1/user/alice" {
		t.Errorf("got path %q, want /api/v1/user/alice", gotPath)
	}
}

func TestRenameUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/user/alice/rename/aliceb" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]HeadscaleUser{"user": {ID: "1", Name: "aliceb"}})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := renameUser(cfg, "alice", "aliceb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "aliceb" {
		t.Errorf("got name %q, want %q", got.Name, "aliceb")
	}
}
