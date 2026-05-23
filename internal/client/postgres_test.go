package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCreatePostgres(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/postgres.create" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		var body PostgresInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DatabaseUser != "app" {
			t.Errorf("databaseUser = %q", body.DatabaseUser)
		}
		_ = json.NewEncoder(w).Encode(Postgres{
			ID: "pg1", Name: "db", AppName: "db-abc",
			DatabaseName: "app", DatabaseUser: "app", DatabasePassword: "secret",
			ApplicationStatus: "idle",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	pg, err := c.CreatePostgres(context.Background(), PostgresInput{
		Name: "db", AppName: "db", EnvironmentID: "env",
		DockerImage:      "postgres:16",
		DatabaseName:     "app",
		DatabaseUser:     "app",
		DatabasePassword: "secret",
	})
	if err != nil {
		t.Fatalf("CreatePostgres() error = %v", err)
	}
	if pg.ID != "pg1" || pg.AppName != "db-abc" {
		t.Errorf("pg = %+v", pg)
	}
}

func TestGetPostgres_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	_, err := c.GetPostgres(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false, want true (err = %v)", err)
	}
}

func TestWaitForPostgresDeployment_Done(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		status := "running"
		if n >= 2 {
			status = "done"
		}
		_ = json.NewEncoder(w).Encode(Postgres{ID: "pg1", ApplicationStatus: status})
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	// statusFn closure pattern mirrors how the provider helper calls it.
	statusFn := func(ctx context.Context) (string, error) {
		pg, err := c.GetPostgres(ctx, "pg1")
		if err != nil {
			return "", err
		}
		return pg.ApplicationStatus, nil
	}
	got, err := statusFn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "running" {
		t.Errorf("first status = %q, want running", got)
	}
	got, _ = statusFn(context.Background())
	got, _ = statusFn(context.Background())
	if got != "done" {
		t.Errorf("third status = %q, want done", got)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}
