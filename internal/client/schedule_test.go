package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSchedule_Application(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/schedule.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body ScheduleInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ScheduleType != "application" || body.ApplicationID != "app1" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Schedule{
			ID:             "s1",
			Name:           body.Name,
			CronExpression: body.CronExpression,
			Command:        body.Command,
			ShellType:      "bash",
			ScheduleType:   body.ScheduleType,
			AppName:        "schedule-foo-bar",
			Enabled:        true,
			ApplicationID:  body.ApplicationID,
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")

	s, err := c.CreateSchedule(context.Background(), ScheduleInput{
		Name:           "warmup",
		CronExpression: "*/15 * * * *",
		Command:        "echo hi",
		ScheduleType:   "application",
		ApplicationID:  "app1",
	})
	if err != nil {
		t.Fatalf("CreateSchedule() error = %v", err)
	}
	if s.ID != "s1" || s.AppName == "" {
		t.Errorf("s = %+v", s)
	}
}

func TestCreateSchedule_DokployServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body ScheduleInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ScheduleType != "dokploy-server" || body.ApplicationID != "" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Schedule{ID: "s2", Name: body.Name, ScheduleType: body.ScheduleType})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")

	s, err := c.CreateSchedule(context.Background(), ScheduleInput{
		Name:           "host-job",
		CronExpression: "0 0 * * *",
		Command:        "echo",
		ScheduleType:   "dokploy-server",
	})
	if err != nil {
		t.Fatalf("CreateSchedule() error = %v", err)
	}
	if s.ID != "s2" {
		t.Errorf("s = %+v", s)
	}
}

func TestGetSchedule_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetSchedule(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
