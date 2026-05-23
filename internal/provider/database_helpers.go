package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"golang.org/x/crypto/ssh"
)

// Default values shared by every database resource's deploy lifecycle.
const (
	defaultDatabaseTimeout = 10 * time.Minute
	databasePollInterval   = 5 * time.Second
)

// slugify turns a display name into a Docker-safe base name. Dokploy appends
// its own random suffix, so this only needs to be a valid prefix.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "app"
	}
	return out
}

// passwordCharset is intentionally alphanumeric-only so generated passwords
// are safe in URL-encoded connection strings without escaping.
const passwordCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// generatePassword returns a 32-character cryptographically random password
// drawn from passwordCharset.
func generatePassword() string {
	max := big.NewInt(int64(len(passwordCharset)))
	var b strings.Builder
	b.Grow(32)
	for i := 0; i < 32; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// crypto/rand.Reader does not fail in practice; if it does, panic
			// so the caller sees the failure rather than getting a weak value.
			panic(fmt.Sprintf("generatePassword: crypto/rand failed: %v", err))
		}
		b.WriteByte(passwordCharset[n.Int64()])
	}
	return b.String()
}

// resolvePassword returns the configured plan value, or a freshly generated
// password when the plan value is null/unknown/empty. Used at Create time by
// every database resource.
func resolvePassword(plan types.String) string {
	if plan.IsNull() || plan.IsUnknown() || plan.ValueString() == "" {
		return generatePassword()
	}
	return plan.ValueString()
}

// generateSSHKeyPair generates a 4096-bit RSA SSH key pair. The private key is
// PEM-encoded (PKCS#1, "RSA PRIVATE KEY"); the public key is OpenSSH-format
// ("ssh-rsa AAAA... <comment>\n"). Used by dokploy_ssh_key when the user
// omits the key inputs.
func generateSSHKeyPair(comment string) (privatePEM, publicOpenSSH string, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", fmt.Errorf("generating RSA key: %w", err)
	}
	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	privatePEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	}))

	pub, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("encoding public key: %w", err)
	}
	publicOpenSSH = strings.TrimRight(string(ssh.MarshalAuthorizedKey(pub)), "\n")
	if comment != "" {
		publicOpenSSH += " " + comment
	}
	publicOpenSSH += "\n"
	return privatePEM, publicOpenSSH, nil
}

// deployAndWait triggers a deploy via deployFn, then polls statusFn at the
// given interval until it returns "done" (success), "error" (failure), or
// ctx is cancelled. Pass timeout to bound the overall wait independently of
// ctx; pass 0 to use only ctx.
func deployAndWait(
	ctx context.Context,
	deployFn func(context.Context) error,
	statusFn func(context.Context) (string, error),
	interval time.Duration,
	timeout time.Duration,
) error {
	if err := deployFn(ctx); err != nil {
		return fmt.Errorf("triggering deploy: %w", err)
	}

	pollCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		pollCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		status, err := statusFn(pollCtx)
		if err != nil {
			return fmt.Errorf("reading deploy status: %w", err)
		}
		switch status {
		case "done":
			return nil
		case "error":
			return fmt.Errorf("deployment failed (status=error); check deploy logs in the Dokploy dashboard")
		}
		select {
		case <-pollCtx.Done():
			return fmt.Errorf("timed out or cancelled waiting for deployment: %w", pollCtx.Err())
		case <-ticker.C:
		}
	}
}
