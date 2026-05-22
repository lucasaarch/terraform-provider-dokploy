package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateEnvironment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environment.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body EnvironmentInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "staging" || body.ProjectID != "p1" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Environment{ID: "env1", Name: "staging", ProjectID: "p1"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	e, err := c.CreateEnvironment(context.Background(), EnvironmentInput{
		Name: "staging", ProjectID: "p1", Env: "LOG_LEVEL=debug",
	})
	if err != nil {
		t.Fatalf("CreateEnvironment() error = %v", err)
	}
	if e.ID != "env1" {
		t.Errorf("env.ID = %q, want env1", e.ID)
	}
}

func TestGetEnvironment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("environmentId") != "env1" {
			t.Errorf("environmentId = %q", r.URL.Query().Get("environmentId"))
		}
		_ = json.NewEncoder(w).Encode(Environment{ID: "env1", Name: "staging", Env: "LOG_LEVEL=debug"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	e, err := c.GetEnvironment(context.Background(), "env1")
	if err != nil {
		t.Fatalf("GetEnvironment() error = %v", err)
	}
	if e.Env != "LOG_LEVEL=debug" {
		t.Errorf("env.Env = %q", e.Env)
	}
}
