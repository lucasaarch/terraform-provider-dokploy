package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateCompose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/compose.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body ComposeInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "monitoring" || body.EnvironmentID != "env1" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Compose{
			ID:            "co1",
			Name:          "monitoring",
			AppName:       "monitoring-abc",
			EnvironmentID: "env1",
			ComposeStatus: "idle",
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	co, err := c.CreateCompose(context.Background(), ComposeInput{
		Name:          "monitoring",
		AppName:       "monitoring",
		EnvironmentID: "env1",
	})
	if err != nil {
		t.Fatalf("CreateCompose() error = %v", err)
	}
	if co.ID != "co1" {
		t.Errorf("co = %+v", co)
	}
}

func TestGetCompose_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetCompose(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
