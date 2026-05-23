package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateMariadb(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mariadb.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body MariadbInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DatabaseRootPassword != "rootpw" {
			t.Errorf("databaseRootPassword = %q", body.DatabaseRootPassword)
		}
		_ = json.NewEncoder(w).Encode(Mariadb{ID: "ma1", AppName: "db-abc"})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	ma, err := c.CreateMariadb(context.Background(), MariadbInput{
		Name: "db", AppName: "db", EnvironmentID: "env",
		DockerImage:          "mariadb:11",
		DatabaseName:         "app",
		DatabaseUser:         "app",
		DatabasePassword:     "pw",
		DatabaseRootPassword: "rootpw",
	})
	if err != nil {
		t.Fatalf("CreateMariadb() error = %v", err)
	}
	if ma.ID != "ma1" {
		t.Errorf("ma = %+v", ma)
	}
}

func TestGetMariadb_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetMariadb(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
