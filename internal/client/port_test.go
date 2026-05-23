package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreatePort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/port.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body PortInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.PublishedPort != 8080 || body.TargetPort != 80 {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Port{
			ID:            "p1",
			ApplicationID: body.ApplicationID,
			PublishedPort: body.PublishedPort,
			TargetPort:    body.TargetPort,
			Protocol:      "tcp",
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	p, err := c.CreatePort(context.Background(), PortInput{
		ApplicationID: "app1",
		PublishedPort: 8080,
		TargetPort:    80,
		Protocol:      "tcp",
	})
	if err != nil {
		t.Fatalf("CreatePort() error = %v", err)
	}
	if p.ID != "p1" {
		t.Errorf("p = %+v", p)
	}
}

func TestGetPort_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetPort(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
