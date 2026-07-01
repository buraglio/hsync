package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetNode(t *testing.T) {
	want := HeadscaleNode{ID: "7", GivenName: "mynode"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("got method %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/node/7" {
			t.Errorf("got path %q, want /api/v1/node/7", r.URL.Path)
		}
		json.NewEncoder(w).Encode(headscaleNodeResponse{Node: want})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := getNode(cfg, "7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "7" {
		t.Errorf("got ID %q, want %q", got.ID, "7")
	}
	if got.GivenName != "mynode" {
		t.Errorf("got GivenName %q, want %q", got.GivenName, "mynode")
	}
}

func TestDeleteNode(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	if err := deleteNode(cfg, "7"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("got method %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v1/node/7" {
		t.Errorf("got path %q, want /api/v1/node/7", gotPath)
	}
}

func TestExpireNode(t *testing.T) {
	want := HeadscaleNode{ID: "7", GivenName: "mynode"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("got method %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/node/7/expire" {
			t.Errorf("got path %q, want /api/v1/node/7/expire", r.URL.Path)
		}
		json.NewEncoder(w).Encode(headscaleNodeResponse{Node: want})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := expireNode(cfg, "7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "7" {
		t.Errorf("got ID %q, want %q", got.ID, "7")
	}
}

func TestMoveNode(t *testing.T) {
	want := HeadscaleNode{ID: "7", GivenName: "mynode"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("got method %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/node/7/user" {
			t.Errorf("got path %q, want /api/v1/node/7/user", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"user":"bob"`) {
			t.Errorf("body %q does not contain {\"user\":\"bob\"}", string(b))
		}
		json.NewEncoder(w).Encode(headscaleNodeResponse{Node: want})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := moveNode(cfg, "7", "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "7" {
		t.Errorf("got ID %q, want %q", got.ID, "7")
	}
}
