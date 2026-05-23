package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateMount_Bind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mounts.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body MountInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Type != "bind" || body.HostPath != "/srv/data" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Mount{ID: "m1", Type: "bind", MountPath: body.MountPath, HostPath: body.HostPath})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	m, err := c.CreateMount(context.Background(), MountInput{
		ServiceID: "app1",
		Type:      "bind",
		MountPath: "/data",
		HostPath:  "/srv/data",
	})
	if err != nil {
		t.Fatalf("CreateMount() error = %v", err)
	}
	if m.ID != "m1" {
		t.Errorf("m = %+v", m)
	}
}

func TestCreateMount_Volume(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body MountInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Type != "volume" || body.VolumeName != "datavol" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Mount{ID: "m2", Type: "volume", VolumeName: body.VolumeName})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	m, err := c.CreateMount(context.Background(), MountInput{
		ServiceID:  "app1",
		Type:       "volume",
		MountPath:  "/data",
		VolumeName: "datavol",
	})
	if err != nil {
		t.Fatalf("CreateMount() error = %v", err)
	}
	if m.ID != "m2" {
		t.Errorf("m = %+v", m)
	}
}

func TestCreateMount_File(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body MountInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Type != "file" || body.Content != "hello\n" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Mount{ID: "m3", Type: "file"})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	m, err := c.CreateMount(context.Background(), MountInput{
		ServiceID: "app1",
		Type:      "file",
		MountPath: "/etc/config.yml",
		Content:   "hello\n",
	})
	if err != nil {
		t.Fatalf("CreateMount() error = %v", err)
	}
	if m.ID != "m3" {
		t.Errorf("m = %+v", m)
	}
}
