package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateMongo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mongo.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"mongoId":"mo1","appName":"db-abc","databaseUser":"root"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	mo, err := c.CreateMongo(context.Background(), MongoInput{
		Name: "db", AppName: "db", EnvironmentID: "env",
		DockerImage:      "mongo:7",
		DatabaseUser:     "root",
		DatabasePassword: "pw",
	})
	if err != nil {
		t.Fatalf("CreateMongo() error = %v", err)
	}
	if mo.ID != "mo1" {
		t.Errorf("mo = %+v", mo)
	}
}

func TestGetMongo_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetMongo(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
