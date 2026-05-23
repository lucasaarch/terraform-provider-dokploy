package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateDestination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/destination.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body DestinationInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Provider != "digital_ocean" {
			t.Errorf("provider = %q", body.Provider)
		}
		_ = json.NewEncoder(w).Encode(Destination{
			ID:              "d1",
			Name:            "prod-backups",
			Provider:        "digital_ocean",
			Bucket:          "my-bucket",
			Endpoint:        "https://sfo3.digitaloceanspaces.com",
			AccessKey:       "AK",
			SecretAccessKey: "SK",
			OrganizationID:  "org1",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	d, err := c.CreateDestination(context.Background(), DestinationInput{
		Name:            "prod-backups",
		Provider:        "digital_ocean",
		Bucket:          "my-bucket",
		Endpoint:        "https://sfo3.digitaloceanspaces.com",
		AccessKey:       "AK",
		SecretAccessKey: "SK",
	})
	if err != nil {
		t.Fatalf("CreateDestination() error = %v", err)
	}
	if d.ID != "d1" || d.Name != "prod-backups" {
		t.Errorf("d = %+v", d)
	}
}

func TestGetDestination_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetDestination(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
