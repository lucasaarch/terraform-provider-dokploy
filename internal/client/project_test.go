package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/project.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body ProjectInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "web" {
			t.Errorf("body = %+v", body)
		}
		// project.create returns a {project, environment} envelope.
		_, _ = w.Write([]byte(`{
			"project": {"projectId": "p1", "name": "web", "organizationId": "org1"},
			"environment": {"environmentId": "env-prod", "name": "production", "isDefault": true}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	p, err := c.CreateProject(context.Background(), ProjectInput{Name: "web"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if p.ID != "p1" || p.ProductionEnvironmentID() != "env-prod" {
		t.Errorf("project = %+v", p)
	}
}

func TestGetProject_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	_, err := c.GetProject(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false, want true (err = %v)", err)
	}
}
