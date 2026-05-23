package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/server.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body ServerInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ServerType != "deploy" || body.SshKeyID != "sk1" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Server{
			ID:             "srv1",
			Name:           body.Name,
			Description:    body.Description,
			IPAddress:      body.IPAddress,
			Port:           body.Port,
			Username:       body.Username,
			SshKeyID:       body.SshKeyID,
			ServerType:     body.ServerType,
			OrganizationID: "org1",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	s, err := c.CreateServer(context.Background(), ServerInput{
		Name:        "worker",
		Description: "",
		IPAddress:   "1.2.3.4",
		Port:        22,
		Username:    "root",
		SshKeyID:    "sk1",
		ServerType:  "deploy",
	})
	if err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	if s.ID != "srv1" {
		t.Errorf("s = %+v", s)
	}
}

func TestGetServer_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetServer(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
