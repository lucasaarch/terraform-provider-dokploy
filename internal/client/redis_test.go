package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateRedis(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/redis.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"redisId":"re1","appName":"cache-abc","databasePassword":"pw"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	re, err := c.CreateRedis(context.Background(), RedisInput{
		Name: "cache", AppName: "cache", EnvironmentID: "env",
		DockerImage:      "redis:7",
		DatabasePassword: "pw",
	})
	if err != nil {
		t.Fatalf("CreateRedis() error = %v", err)
	}
	if re.ID != "re1" {
		t.Errorf("re = %+v", re)
	}
}

func TestGetRedis_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetRedis(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
