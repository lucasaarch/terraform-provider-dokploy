package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreateBackup verifies the diff-based id discovery: list backups on the
// parent, call backup.create with an empty 200 response, list again, and
// return the new backup. The mock server flips a flag on the second list call.
func TestCreateBackup(t *testing.T) {
	createCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/postgres.one":
			if r.URL.Query().Get("postgresId") != "pg1" {
				t.Errorf("postgresId = %q", r.URL.Query().Get("postgresId"))
			}
			out := Postgres{ID: "pg1", Name: "db"}
			if createCalled {
				out.Backups = []Backup{{ID: "b1", Schedule: "0 3 * * *", DatabaseType: "postgres"}}
			}
			_ = json.NewEncoder(w).Encode(out)
		case "/api/backup.create":
			var body BackupInput
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.PostgresID == nil || *body.PostgresID != "pg1" {
				t.Errorf("postgresId not set: body = %+v", body)
			}
			createCalled = true
			w.WriteHeader(http.StatusOK)
			// Empty body — matches real API.
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "k")

	b, err := c.CreateBackup(context.Background(), BackupInput{
		Schedule:      "0 3 * * *",
		Prefix:        "postgres/app/",
		DestinationID: "d1",
		Database:      "pg1",
		DatabaseType:  "postgres",
	})
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	if b.ID != "b1" {
		t.Errorf("b = %+v", b)
	}
}

func TestGetBackup_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetBackup(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}

func TestSetTypedID(t *testing.T) {
	cases := map[string]func(*BackupInput) bool{
		"postgres":   func(in *BackupInput) bool { return in.PostgresID != nil && *in.PostgresID == "x" },
		"mysql":      func(in *BackupInput) bool { return in.MysqlID != nil && *in.MysqlID == "x" },
		"mariadb":    func(in *BackupInput) bool { return in.MariadbID != nil && *in.MariadbID == "x" },
		"mongo":      func(in *BackupInput) bool { return in.MongoID != nil && *in.MongoID == "x" },
		"libsql":     func(in *BackupInput) bool { return in.LibsqlID != nil && *in.LibsqlID == "x" },
		"web-server": func(in *BackupInput) bool { return in.PostgresID == nil && in.MysqlID == nil }, // none set
	}
	for typ, check := range cases {
		in := &BackupInput{Database: "x", DatabaseType: typ}
		if err := in.SetTypedID(); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
		if !check(in) {
			t.Errorf("%s: typed id not set correctly: %+v", typ, in)
		}
	}
}
