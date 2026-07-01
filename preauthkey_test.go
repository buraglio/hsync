package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListPreAuthKeys(t *testing.T) {
	want := []HeadscalePreAuthKey{
		{ID: "1", Key: "abc123", User: "alice", Reusable: false, Used: false},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/preauthkey" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("user") != "alice" {
			t.Errorf("expected user=alice, got %q", r.URL.Query().Get("user"))
		}
		json.NewEncoder(w).Encode(map[string][]HeadscalePreAuthKey{"preAuthKeys": want})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := listPreAuthKeys(cfg, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Key != "abc123" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestCreatePreAuthKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/preauthkey" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["user"] != "alice" {
			t.Errorf("expected user=alice in body, got %v", body["user"])
		}
		key := HeadscalePreAuthKey{ID: "2", Key: "newkey456", User: "alice", Reusable: true}
		json.NewEncoder(w).Encode(map[string]HeadscalePreAuthKey{"preAuthKey": key})
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	got, err := createPreAuthKey(cfg, "alice", true, false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Key != "newkey456" {
		t.Errorf("got key %q, want %q", got.Key, "newkey456")
	}
}

func TestExpirePreAuthKey(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/preauthkey/expire" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &Config{HeadscaleURL: srv.URL, HeadscaleAPIKey: "test"}
	if err := expirePreAuthKey(cfg, "alice", "abc123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["user"] != "alice" || gotBody["key"] != "abc123" {
		t.Errorf("unexpected body: %v", gotBody)
	}
}
