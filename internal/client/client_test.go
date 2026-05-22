package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDo_GETSendsAPIKeyAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "secret" {
			t.Errorf("x-api-key = %q, want %q", got, "secret")
		}
		if r.URL.Path != "/api/project.one" {
			t.Errorf("path = %q, want /api/project.one", r.URL.Path)
		}
		if got := r.URL.Query().Get("projectId"); got != "p1" {
			t.Errorf("query projectId = %q, want p1", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "demo"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	var out struct {
		Name string `json:"name"`
	}
	q := url.Values{"projectId": {"p1"}}
	if err := c.do(context.Background(), http.MethodGet, "project.one", nil, q, &out); err != nil {
		t.Fatalf("do() error = %v", err)
	}
	if out.Name != "demo" {
		t.Errorf("out.Name = %q, want demo", out.Name)
	}
}

func TestDo_ErrorStatusReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	err := c.do(context.Background(), http.MethodGet, "project.one", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false, want true for err %v", err)
	}
}

func TestDo_POSTSendsJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "demo" {
			t.Errorf("body name = %q, want demo", body["name"])
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"projectId": "p1"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	var out struct {
		ProjectID string `json:"projectId"`
	}
	in := map[string]string{"name": "demo"}
	if err := c.do(context.Background(), http.MethodPost, "project.create", in, nil, &out); err != nil {
		t.Fatalf("do() error = %v", err)
	}
	if out.ProjectID != "p1" {
		t.Errorf("out.ProjectID = %q, want p1", out.ProjectID)
	}
}
