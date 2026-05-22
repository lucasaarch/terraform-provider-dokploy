package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreateApplication(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/application.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body ApplicationInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.AppName != "api" {
			t.Errorf("appName = %q, want api (required by the API)", body.AppName)
		}
		_ = json.NewEncoder(w).Encode(Application{ID: "app1", Name: "api", AppName: "api-abc123"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	app, err := c.CreateApplication(context.Background(),
		ApplicationInput{Name: "api", AppName: "api", EnvironmentID: "env1"})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if app.ID != "app1" || app.AppName != "api-abc123" {
		t.Errorf("app = %+v", app)
	}
}

func TestWaitForDeployment_PollsUntilDone(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		status := "running"
		if n >= 3 {
			status = "done"
		}
		_ = json.NewEncoder(w).Encode(Application{ID: "app1", ApplicationStatus: status})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	err := c.WaitForDeployment(context.Background(), "app1", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForDeployment() error = %v", err)
	}
	if atomic.LoadInt32(&calls) < 3 {
		t.Errorf("polled %d times, want >= 3", calls)
	}
}

func TestWaitForDeployment_ErrorStatusFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Application{ID: "app1", ApplicationStatus: "error"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	err := c.WaitForDeployment(context.Background(), "app1", 1*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for failed deployment, got nil")
	}
}
