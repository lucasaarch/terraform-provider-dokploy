package provider

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"":            "app",
		"  ":          "app",
		"Hello":       "hello",
		"Hello-World": "hello-world",
		"app name":    "app-name",
		"weird!@#":    "weird",
		"-leading-":   "leading",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGeneratePassword_LengthAndCharset(t *testing.T) {
	pw := generatePassword()
	if len(pw) != 32 {
		t.Fatalf("len(pw) = %d, want 32", len(pw))
	}
	ok := regexp.MustCompile(`^[a-zA-Z0-9]{32}$`).MatchString(pw)
	if !ok {
		t.Errorf("password %q has chars outside [a-zA-Z0-9]", pw)
	}
}

func TestGeneratePassword_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		pw := generatePassword()
		if seen[pw] {
			t.Fatalf("duplicate password generated: %q", pw)
		}
		seen[pw] = true
	}
}

func TestDeployAndWait_TerminalDone(t *testing.T) {
	calls := 0
	statusFn := func(_ context.Context) (string, error) {
		calls++
		if calls >= 3 {
			return "done", nil
		}
		return "running", nil
	}
	deployFn := func(_ context.Context) error { return nil }

	err := deployAndWait(context.Background(), deployFn, statusFn, 1*time.Millisecond, 5*time.Second)
	if err != nil {
		t.Fatalf("deployAndWait() error = %v", err)
	}
	if calls < 3 {
		t.Errorf("statusFn called %d times, want >= 3", calls)
	}
}

func TestDeployAndWait_TerminalError(t *testing.T) {
	deployFn := func(_ context.Context) error { return nil }
	statusFn := func(_ context.Context) (string, error) { return "error", nil }

	err := deployAndWait(context.Background(), deployFn, statusFn, 1*time.Millisecond, 5*time.Second)
	if err == nil {
		t.Fatal("expected error for failed deploy, got nil")
	}
}

func TestDeployAndWait_DeployFnError(t *testing.T) {
	deployFn := func(_ context.Context) error { return errors.New("boom") }
	statusFn := func(_ context.Context) (string, error) { return "done", nil }

	err := deployAndWait(context.Background(), deployFn, statusFn, 1*time.Millisecond, 5*time.Second)
	if err == nil || err.Error() != "triggering deploy: boom" {
		t.Errorf("err = %v, want triggering deploy: boom", err)
	}
}

func TestGenerateSSHKeyPair_Format(t *testing.T) {
	priv, pub, err := generateSSHKeyPair("test-key")
	if err != nil {
		t.Fatalf("generateSSHKeyPair() error = %v", err)
	}
	if !strings.HasPrefix(priv, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Errorf("private key prefix wrong: %q", priv[:64])
	}
	if !strings.HasSuffix(strings.TrimRight(priv, "\n"), "-----END RSA PRIVATE KEY-----") {
		t.Errorf("private key suffix wrong")
	}
	if !strings.HasPrefix(pub, "ssh-rsa ") {
		t.Errorf("public key prefix wrong: %q", pub[:32])
	}
	if !strings.Contains(pub, "test-key") {
		t.Errorf("public key missing name comment: %q", pub)
	}
}

func TestGenerateSSHKeyPair_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		priv, _, err := generateSSHKeyPair("k")
		if err != nil {
			t.Fatal(err)
		}
		if seen[priv] {
			t.Fatal("duplicate private key generated")
		}
		seen[priv] = true
	}
}
