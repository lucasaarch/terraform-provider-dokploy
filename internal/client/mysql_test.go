package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateMysql(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mysql.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body MysqlInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DatabaseRootPassword != "rootpw" {
			t.Errorf("databaseRootPassword = %q", body.DatabaseRootPassword)
		}
		_ = json.NewEncoder(w).Encode(Mysql{
			ID: "my1", AppName: "db-abc",
			DatabaseName: "app", DatabaseUser: "app",
			DatabasePassword: "pw", DatabaseRootPassword: "rootpw",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	my, err := c.CreateMysql(context.Background(), MysqlInput{
		Name: "db", AppName: "db", EnvironmentID: "env",
		DockerImage:          "mysql:8",
		DatabaseName:         "app",
		DatabaseUser:         "app",
		DatabasePassword:     "pw",
		DatabaseRootPassword: "rootpw",
	})
	if err != nil {
		t.Fatalf("CreateMysql() error = %v", err)
	}
	if my.ID != "my1" {
		t.Errorf("my = %+v", my)
	}
}

func TestGetMysql_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetMysql(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
