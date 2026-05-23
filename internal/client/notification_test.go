package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSlackNotification(t *testing.T) {
	var createCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/notification.all":
			// Return empty before create, one item after create
			if !createCalled {
				_ = json.NewEncoder(w).Encode([]Notification{})
			} else {
				_ = json.NewEncoder(w).Encode([]Notification{
					{ID: "n1", Name: "alerts", NotificationType: "slack"},
				})
			}
		case "/api/notification.createSlack":
			var body SlackNotificationInput
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.WebhookURL == "" || body.Channel == "" {
				t.Errorf("body = %+v", body)
			}
			createCalled = true
			// Empty body response as per API spec
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected path = %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	n, err := c.CreateSlackNotification(context.Background(), SlackNotificationInput{
		Name:       "alerts",
		WebhookURL: "https://hooks.slack.com/services/T0/B0/X",
		Channel:    "#deploys",
		EventFlags: EventFlags{
			AppDeploy:       true,
			AppBuildError:   true,
			DatabaseBackup:  true,
			DokployBackup:   true,
			VolumeBackup:    true,
			DokployRestart:  true,
			DockerCleanup:   true,
			ServerThreshold: true,
		},
	})
	if err != nil {
		t.Fatalf("CreateSlackNotification() error = %v", err)
	}
	if n.ID != "n1" {
		t.Errorf("n = %+v", n)
	}
}

func TestGetNotification_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetNotification(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
