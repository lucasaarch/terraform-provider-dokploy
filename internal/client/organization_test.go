package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListOrganizations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/organization.all" {
			t.Errorf("got %s %s, want GET /api/organization.all", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"id":"org1","name":"Acme","slug":"acme"}]`))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	orgs, err := c.ListOrganizations(context.Background())
	if err != nil {
		t.Fatalf("ListOrganizations() error = %v", err)
	}
	if len(orgs) != 1 {
		t.Fatalf("len(orgs) = %d, want 1", len(orgs))
	}
	if orgs[0].ID != "org1" || orgs[0].Name != "Acme" || orgs[0].Slug != "acme" {
		t.Errorf("org = %+v", orgs[0])
	}
}
