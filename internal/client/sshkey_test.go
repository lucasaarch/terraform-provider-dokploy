package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSshKey(t *testing.T) {
	var createCalled bool
	// The real API returns empty body on sshKey.create; we simulate this.
	// CreateSshKey diffs sshKey.all before and after to find the new key.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/sshKey.all":
			if !createCalled {
				// Before create: empty list
				_ = json.NewEncoder(w).Encode([]SshKey{})
			} else {
				// After create: list with the new key
				_ = json.NewEncoder(w).Encode([]SshKey{{
					ID:             "sk1",
					Name:           "worker",
					PublicKey:      "ssh-rsa AAAA",
					PrivateKey:     "-----BEGIN RSA PRIVATE KEY-----\n...",
					OrganizationID: "org1",
				}})
			}
		case "/api/sshKey.create":
			var body SshKeyInput
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Name != "worker" || body.OrganizationID != "org1" {
				t.Errorf("body = %+v", body)
			}
			if body.PublicKey == "" || body.PrivateKey == "" {
				t.Errorf("keys not sent: pub=%q priv=%q", body.PublicKey, body.PrivateKey)
			}
			createCalled = true
			// Empty body — real API behaviour
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected path = %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	k, err := c.CreateSshKey(context.Background(), SshKeyInput{
		Name:           "worker",
		OrganizationID: "org1",
		PublicKey:      "ssh-rsa AAAA",
		PrivateKey:     "-----BEGIN RSA PRIVATE KEY-----\n...",
	})
	if err != nil {
		t.Fatalf("CreateSshKey() error = %v", err)
	}
	if k.ID != "sk1" {
		t.Errorf("k = %+v", k)
	}
}

func TestGetSshKey_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetSshKey(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
