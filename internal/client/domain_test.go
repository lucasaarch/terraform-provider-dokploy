package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateDomain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/domain.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body DomainInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Host != "api.example.com" {
			t.Errorf("host = %q", body.Host)
		}
		_ = json.NewEncoder(w).Encode(Domain{ID: "d1", Host: "api.example.com"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	d, err := c.CreateDomain(context.Background(), DomainInput{
		Host: "api.example.com", ApplicationID: "app1", Port: 8080,
	})
	if err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}
	if d.ID != "d1" {
		t.Errorf("domain.ID = %q, want d1", d.ID)
	}
}
