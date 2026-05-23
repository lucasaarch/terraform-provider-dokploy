# Dokploy Servers & SSH Keys (v0.4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `dokploy_ssh_key`, `dokploy_server`, and `dokploy_server_schedule` resources, plus an Optional+ForceNew `server_id` attribute on application + the five database resources, so users can deploy onto managed remote servers. Ship as v0.4.0.

**Architecture:** Three new resources follow the existing two-layer pattern (`internal/client/<router>.go` typed clients + `internal/provider/<resource>.go` Terraform layer). SSH-key generation lives in `internal/provider/database_helpers.go` alongside the existing `generatePassword`. The 6-resource modification is mechanical: add a `ServerID *string` field to each client struct/input and a `server_id` attribute (ForceNew) to each resource schema. No API breakage — every new attribute is Optional.

**Tech Stack:** Go 1.26, `terraform-plugin-framework`, `golang.org/x/crypto/ssh` (promoted from transitive to direct dependency for SSH key marshaling).

**Spec:** `docs/superpowers/specs/2026-05-23-dokploy-servers-v0.4-design.md`

---

## Conventions for every task

- TDD: failing test first, see it fail, implement, see it pass, commit.
- Run `gofmt -w .` before every commit.
- Commit messages: conventional commits (`feat:`, `test:`, `chore:`, `docs:`).
- Unit tests use `httptest` and need no network. Acceptance tests (`TestAcc*`) require `TF_ACC=1` with `DOKPLOY_ENDPOINT` and `DOKPLOY_API_KEY` set (`source .dokploy-test-env`).
- End every commit message body with: `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>`.
- Acceptance tests hit the user's live Dokploy instance. All test resource names use `tf-acc-` prefix. After running, confirm the instance has no `tf-acc-*` items left.
- `internal/client/API.md` is the source of truth for endpoint shapes. Where this plan's code differs from API.md, **API.md wins** — adjust accordingly.

---

## Task 1: Verify sshKey, server, and serverId support on existing routers

Probe the live API for the missing details before any code is written.

**Files:**
- Modify: `internal/client/API.md` (append sections)

- [ ] **Step 1: Load credentials**

```bash
cd /Users/lukearch/Projects/My/dokploy-terraform-provider
source .dokploy-test-env
```

- [ ] **Step 2: Probe `sshKey.*` and `server.*` endpoint names (create/one/update/delete/remove)**

```bash
# We know sshKey.create requires name/privateKey/publicKey/organizationId.
# We know server.create requires name/description/ipAddress/port/username/sshKeyId/serverType (deploy|build).
# Find: which delete verb (.delete vs .remove); whether sshKey.update exists.

for path in sshKey.one sshKey.update sshKey.delete sshKey.remove server.one server.update server.delete server.remove; do
  /usr/bin/curl -s -m 10 -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" -d '{}' \
    -o /tmp/x -w "$path: HTTP %{http_code}\n" "$DOKPLOY_ENDPOINT/api/$path"
done
```

A 400 (Zod validation error) means the route exists. A 404 means the route doesn't exist. Record which delete verb and update verb exist for each.

- [ ] **Step 3: Probe `sshKey.create` and inspect the full response shape**

```bash
ORG_ID=$(/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" "$DOKPLOY_ENDPOINT/api/organization.all" \
  | /opt/homebrew/bin/python3 -c "import json,sys; print(json.load(sys.stdin)[0]['id'])")
echo "org id: $ORG_ID"

# Generate a throwaway keypair locally to send.
ssh-keygen -t rsa -b 2048 -f /tmp/tf-probe-key -N "" -q
PRIVATE=$(cat /tmp/tf-probe-key | /opt/homebrew/bin/python3 -c "import sys,json; print(json.dumps(sys.stdin.read()))")
PUBLIC=$(cat /tmp/tf-probe-key.pub | /opt/homebrew/bin/python3 -c "import sys,json; print(json.dumps(sys.stdin.read()))")

/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"name\":\"tf-probe-sshkey\",\"organizationId\":\"$ORG_ID\",\"publicKey\":$PUBLIC,\"privateKey\":$PRIVATE}" \
  "$DOKPLOY_ENDPOINT/api/sshKey.create" | /opt/homebrew/bin/python3 -m json.tool
```

Record the full create response. Note in particular whether `privateKey` and `publicKey` come back in the response (or only the IDs).

- [ ] **Step 4: Inspect `sshKey.one`**

```bash
SSH_KEY_ID=$(<the id from previous response>)
/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" "$DOKPLOY_ENDPOINT/api/sshKey.one?sshKeyId=$SSH_KEY_ID" \
  | /opt/homebrew/bin/python3 -m json.tool
```

Critical: does `sshKey.one` return `privateKey` in plaintext? Yes → Read overwrites state from API. No → preserve state value (same pattern as `registry_password`).

- [ ] **Step 5: Probe `sshKey.update` field requirements**

```bash
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"sshKeyId\":\"$SSH_KEY_ID\",\"name\":\"tf-probe-sshkey-renamed\"}" \
  "$DOKPLOY_ENDPOINT/api/sshKey.update" -w "\nHTTP %{http_code}\n"

# Also probe: can update change privateKey/publicKey?
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"sshKeyId\":\"$SSH_KEY_ID\",\"privateKey\":\"NEW\"}" \
  "$DOKPLOY_ENDPOINT/api/sshKey.update" -w "\nHTTP %{http_code}\n"
```

Record whether name/privateKey/publicKey are independently updatable, and any required fields the update endpoint demands.

- [ ] **Step 6: Probe each DB router's create endpoint for `serverId` support**

```bash
# We use the empty-body Zod-error probe (it returns the schema's required fields).
for db in postgres mysql mariadb mongo redis; do
  echo "== $db.create accepted fields =="
  /usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
    -d '{"name":"tf-probe","appName":"tf-probe","environmentId":"nope","dockerImage":"none","serverId":"nope"}' \
    "$DOKPLOY_ENDPOINT/api/$db.create" \
    | /opt/homebrew/bin/python3 -c "import json,sys; d=json.load(sys.stdin); print('zodError:', d.get('data',{}).get('zodError'))"
done
```

If `serverId` triggers a "unknown field" / "unrecognized key" Zod error, the DB doesn't support it. If the error is about other fields (like a foreign-key violation on `serverId`), it does support it. Record per-DB.

- [ ] **Step 7: Clean up the probe sshKey**

```bash
# Use whichever delete verb worked in Step 2 — assumed .remove.
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"sshKeyId\":\"$SSH_KEY_ID\"}" "$DOKPLOY_ENDPOINT/api/sshKey.remove" -w "\nHTTP %{http_code}\n"
rm /tmp/tf-probe-key /tmp/tf-probe-key.pub
```

Confirm no `tf-probe-*` items remain via `sshKey.all`.

- [ ] **Step 8: Append three new sections to `internal/client/API.md`**

After the existing `## schedule.*` section, add:

- `## sshKey.*` — methods (create, one, update, remove), full request body, full response shape of `sshKey.one`, whether the response includes `privateKey`/`publicKey`.
- `## server.*` — methods (create, one, update, remove), full request body (with `serverType` enum noted: `"deploy"|"build"`), full response shape of `server.one`, note about handshake on create.
- `## server_id field on existing routers` — a short note listing which of the 5 DB routers accept `serverId` on create/update.

Match the depth of the existing v0.3 sections.

- [ ] **Step 9: Commit**

```bash
git add internal/client/API.md
git commit -m "docs: API reference for sshKey and server routers + serverId on databases"
```

---

## Task 2: dokploy_ssh_key resource

**Files:**
- Create: `internal/client/sshkey.go`
- Create: `internal/client/sshkey_test.go`
- Modify: `internal/provider/database_helpers.go` (add `generateSSHKeyPair`)
- Modify: `internal/provider/database_helpers_test.go` (add tests)
- Create: `internal/provider/ssh_key_resource.go`
- Create: `internal/provider/ssh_key_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewSshKeyResource`)
- Modify: `go.mod` (promote `golang.org/x/crypto` to direct dep)

Adjust paths/verbs to match `API.md` (from Task 1) where the plan code differs.

- [ ] **Step 1: Add `golang.org/x/crypto` to go.mod as a direct dependency**

```bash
go get golang.org/x/crypto
```

Verify it appears in the `require` block of `go.mod`.

- [ ] **Step 2: Write failing tests for `generateSSHKeyPair`**

Append to `internal/provider/database_helpers_test.go`:

```go
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
```

The `strings` import is already in the test file (used by `TestSlugify`); no new import.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/provider/ -run TestGenerateSSHKeyPair -v`
Expected: FAIL — `undefined: generateSSHKeyPair`.

- [ ] **Step 4: Implement `generateSSHKeyPair` in `internal/provider/database_helpers.go`**

Add these imports at the top of the file:

```go
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
```

Append the function at the end of the file:

```go
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/provider/ -run TestGenerateSSHKeyPair -v`
Expected: PASS.

- [ ] **Step 6: Write the failing client unit tests**

`internal/client/sshkey_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSshKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sshKey.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body SshKeyInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "worker" || body.OrganizationID != "org1" {
			t.Errorf("body = %+v", body)
		}
		if body.PublicKey == "" || body.PrivateKey == "" {
			t.Errorf("keys not sent: pub=%q priv=%q", body.PublicKey, body.PrivateKey)
		}
		_ = json.NewEncoder(w).Encode(SshKey{
			ID:             "sk1",
			Name:           "worker",
			PublicKey:      body.PublicKey,
			PrivateKey:     body.PrivateKey,
			OrganizationID: "org1",
		})
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
```

- [ ] **Step 7: Run tests to verify they fail**

Run: `go test ./internal/client/ -run SshKey -v` — Expected: FAIL — `undefined: SshKey`.

- [ ] **Step 8: Write `internal/client/sshkey.go`**

> The Dokploy SSH key API has two traits that shape this code:
> 1. **`sshKey.create` returns HTTP 200 with an empty body**, identical to `backup.create` in v0.3. The new key's id is found by listing `sshKey.all` after the call and diffing.
> 2. **`sshKey.update` only accepts `name` and `description`** — the keys themselves are immutable. Trying to update `privateKey`/`publicKey` results in a server error.

```go
package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// SshKey is an SSH key registered in Dokploy.
type SshKey struct {
	ID             string `json:"sshKeyId"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	PublicKey      string `json:"publicKey"`
	PrivateKey     string `json:"privateKey"`
	OrganizationID string `json:"organizationId"`
}

// SshKeyInput is the create payload (name + keys + org + optional description).
type SshKeyInput struct {
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	PublicKey      string `json:"publicKey,omitempty"`
	PrivateKey     string `json:"privateKey,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
}

// SshKeyUpdateInput is the restricted update payload — only name/description.
type SshKeyUpdateInput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// ListSshKeys returns every SSH key visible to the API key.
func (c *Client) ListSshKeys(ctx context.Context) ([]SshKey, error) {
	var out []SshKey
	if err := c.do(ctx, http.MethodGet, "sshKey.all", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSshKey creates an SSH key. The API responds with an empty body, so the
// new key's id is discovered by diffing sshKey.all before and after.
func (c *Client) CreateSshKey(ctx context.Context, in SshKeyInput) (*SshKey, error) {
	before, err := c.ListSshKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing ssh keys before create: %w", err)
	}
	seen := make(map[string]struct{}, len(before))
	for _, k := range before {
		seen[k.ID] = struct{}{}
	}

	if err := c.do(ctx, http.MethodPost, "sshKey.create", in, nil, nil); err != nil {
		return nil, err
	}

	after, err := c.ListSshKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing ssh keys after create: %w", err)
	}
	for i := range after {
		if _, was := seen[after[i].ID]; !was {
			return &after[i], nil
		}
	}
	return nil, fmt.Errorf("sshKey.create returned 200 but no new key found")
}

func (c *Client) GetSshKey(ctx context.Context, id string) (*SshKey, error) {
	var out SshKey
	q := url.Values{"sshKeyId": {id}}
	if err := c.do(ctx, http.MethodGet, "sshKey.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSshKey updates name and/or description. The keys themselves are
// immutable — changing them requires destroying and recreating the resource.
func (c *Client) UpdateSshKey(ctx context.Context, id string, in SshKeyUpdateInput) error {
	payload := struct {
		SshKeyUpdateInput
		ID string `json:"sshKeyId"`
	}{SshKeyUpdateInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "sshKey.update", payload, nil, nil)
}

func (c *Client) DeleteSshKey(ctx context.Context, id string) error {
	payload := map[string]string{"sshKeyId": id}
	return c.do(ctx, http.MethodPost, "sshKey.remove", payload, nil, nil)
}
```

- [ ] **Step 9: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run SshKey -v` — Expected: PASS.

- [ ] **Step 10: Write the failing acceptance test**

`internal/provider/ssh_key_resource_test.go`:

```go
package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSshKeyResource(t *testing.T) {
	suffix := randInt()
	config := func(name string) string {
		return fmt.Sprintf(`
data "dokploy_organization" "current" {
  name = %q
}

resource "dokploy_ssh_key" "test" {
  organization_id = data.dokploy_organization.current.id
  name            = %q
  # private_key/public_key omitted → provider generates 4096-bit RSA.
}`, firstOrgName(t), name)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(fmt.Sprintf("tf-acc-sshkey-%d", suffix)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_ssh_key.test", "id"),
					resource.TestMatchResourceAttr("dokploy_ssh_key.test", "public_key",
						regexp.MustCompile(`^ssh-rsa AAAA[A-Za-z0-9+/=]+`)),
					resource.TestMatchResourceAttr("dokploy_ssh_key.test", "private_key",
						regexp.MustCompile(`(?s)^-----BEGIN RSA PRIVATE KEY-----.*-----END RSA PRIVATE KEY-----`)),
				),
			},
			{
				ResourceName:            "dokploy_ssh_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"private_key"}, // API may or may not return it; safer to ignore
			},
			{
				Config: config(fmt.Sprintf("tf-acc-sshkey-%d-renamed", suffix)),
				Check:  resource.TestCheckResourceAttr("dokploy_ssh_key.test", "name", fmt.Sprintf("tf-acc-sshkey-%d-renamed", suffix)),
			},
		},
	})
}
```

- [ ] **Step 11: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewSshKeyResource`.

- [ ] **Step 12: Write `internal/provider/ssh_key_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &sshKeyResource{}
	_ resource.ResourceWithConfigure   = &sshKeyResource{}
	_ resource.ResourceWithImportState = &sshKeyResource{}
)

type sshKeyResource struct {
	client *client.Client
}

func NewSshKeyResource() resource.Resource { return &sshKeyResource{} }

type sshKeyModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	PublicKey      types.String `tfsdk:"public_key"`
	PrivateKey     types.String `tfsdk:"private_key"`
}

func (r *sshKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (r *sshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An SSH key registered at the Dokploy organization level. Used by `dokploy_server` to authenticate against the remote machine.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organization that owns the SSH key. Reference `data.dokploy_organization.x.id`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name.",
			},
			"public_key": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Public key in OpenSSH format (`ssh-rsa AAAA...`). If omitted, the provider generates a 4096-bit RSA pair. **Immutable**: changing this value forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"private_key": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Private key in PEM PKCS#1 format. If omitted, the provider generates one. **Immutable**: changing this value forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *sshKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *sshKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sshKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priv := plan.PrivateKey.ValueString()
	pub := plan.PublicKey.ValueString()
	if priv == "" || pub == "" {
		generatedPriv, generatedPub, err := generateSSHKeyPair(plan.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error generating SSH key", err.Error())
			return
		}
		if priv == "" {
			priv = generatedPriv
		}
		if pub == "" {
			pub = generatedPub
		}
	}

	k, err := r.client.CreateSshKey(ctx, client.SshKeyInput{
		Name:           plan.Name.ValueString(),
		OrganizationID: plan.OrganizationID.ValueString(),
		PublicKey:      pub,
		PrivateKey:     priv,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating SSH key", err.Error())
		return
	}

	plan.ID = types.StringValue(k.ID)
	plan.PrivateKey = types.StringValue(priv)
	plan.PublicKey = types.StringValue(pub)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *sshKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sshKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	k, err := r.client.GetSshKey(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading SSH key", err.Error())
		return
	}
	state.Name = types.StringValue(k.Name)
	state.OrganizationID = types.StringValue(k.OrganizationID)
	if k.PublicKey != "" {
		state.PublicKey = types.StringValue(k.PublicKey)
	}
	if k.PrivateKey != "" {
		state.PrivateKey = types.StringValue(k.PrivateKey)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *sshKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sshKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only name and description are mutable on sshKey.update. Public/private keys
	// are immutable (ForceNew). The schema enforces this at plan time, so by the
	// time Update is reached only name (and maybe description) actually changed.
	if err := r.client.UpdateSshKey(ctx, plan.ID.ValueString(), client.SshKeyUpdateInput{
		Name: plan.Name.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Error updating SSH key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *sshKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sshKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSshKey(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting SSH key", err.Error())
	}
}

func (r *sshKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 13: Register, build, run acceptance test**

Append `NewSshKeyResource,` to the slice in `internal/provider/provider.go::Resources()`.

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run TestAccSshKeyResource -v -timeout 10m
```

Expected: build clean; acceptance test PASS.

- [ ] **Step 14: Commit**

```bash
git add internal/client/sshkey.go internal/client/sshkey_test.go \
        internal/provider/database_helpers.go internal/provider/database_helpers_test.go \
        internal/provider/ssh_key_resource.go internal/provider/ssh_key_resource_test.go \
        internal/provider/provider.go go.mod go.sum
git commit -m "feat: dokploy_ssh_key resource with client-side RSA 4096 generation"
```

---

## Task 3: dokploy_server resource

**Files:**
- Create: `internal/client/server.go`
- Create: `internal/client/server_test.go`
- Create: `internal/provider/server_resource.go`
- Create: `internal/provider/server_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewServerResource`)

- [ ] **Step 1: Write failing client unit tests**

`internal/client/server_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/server.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body ServerInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ServerType != "deploy" || body.SshKeyID != "sk1" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Server{
			ID:             "srv1",
			Name:           body.Name,
			Description:    body.Description,
			IPAddress:      body.IPAddress,
			Port:           body.Port,
			Username:       body.Username,
			SshKeyID:       body.SshKeyID,
			ServerType:     body.ServerType,
			OrganizationID: "org1",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	s, err := c.CreateServer(context.Background(), ServerInput{
		Name:        "worker",
		Description: "",
		IPAddress:   "1.2.3.4",
		Port:        22,
		Username:    "root",
		SshKeyID:    "sk1",
		ServerType:  "deploy",
	})
	if err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	if s.ID != "srv1" {
		t.Errorf("s = %+v", s)
	}
}

func TestGetServer_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetServer(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Server -v` — Expected: FAIL (note: also matches the schedule's `TestCreateSchedule` etc. — narrow with `TestCreateServer` if needed).

- [ ] **Step 3: Write `internal/client/server.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Server is a remote machine registered as a managed worker.
type Server struct {
	ID             string `json:"serverId"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	IPAddress      string `json:"ipAddress"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	SshKeyID       string `json:"sshKeyId"`
	ServerType     string `json:"serverType"`
	OrganizationID string `json:"organizationId"`
}

// ServerInput is the create/update payload.
type ServerInput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description"`
	IPAddress   string `json:"ipAddress,omitempty"`
	Port        int    `json:"port,omitempty"`
	Username    string `json:"username,omitempty"`
	SshKeyID    string `json:"sshKeyId,omitempty"`
	ServerType  string `json:"serverType,omitempty"`
}

func (c *Client) CreateServer(ctx context.Context, in ServerInput) (*Server, error) {
	var out Server
	if err := c.do(ctx, http.MethodPost, "server.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetServer(ctx context.Context, id string) (*Server, error) {
	var out Server
	q := url.Values{"serverId": {id}}
	if err := c.do(ctx, http.MethodGet, "server.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateServer(ctx context.Context, id string, in ServerInput) error {
	payload := struct {
		ServerInput
		ID string `json:"serverId"`
	}{ServerInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "server.update", payload, nil, nil)
}

func (c *Client) DeleteServer(ctx context.Context, id string) error {
	payload := map[string]string{"serverId": id}
	return c.do(ctx, http.MethodPost, "server.remove", payload, nil, nil)
}
```

> `Description` has no `omitempty` because the API requires the field present (verified — see API.md). Adjust delete verb (`server.delete` vs `.remove`) per Task 1's findings.

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run TestCreateServer -v` and `go test ./internal/client/ -run TestGetServer_NotFound -v` — Expected: PASS.

- [ ] **Step 5: Write the failing acceptance test (opt-in)**

`internal/provider/server_resource_test.go`:

```go
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccServerResource is opt-in: it requires a reachable VM whose
// authorized_keys contains the public key the provider generates. Set the env
// vars below to enable.
func TestAccServerResource(t *testing.T) {
	ip := os.Getenv("DOKPLOY_TEST_SERVER_IP")
	if ip == "" {
		t.Skip("set DOKPLOY_TEST_SERVER_IP, DOKPLOY_TEST_SERVER_USER, DOKPLOY_TEST_SERVER_PORT, and DOKPLOY_TEST_SERVER_PRIVATE_KEY to run.")
	}
	user := os.Getenv("DOKPLOY_TEST_SERVER_USER")
	port := os.Getenv("DOKPLOY_TEST_SERVER_PORT")
	priv := os.Getenv("DOKPLOY_TEST_SERVER_PRIVATE_KEY") // private key whose .pub is in authorized_keys

	suffix := randInt()
	config := fmt.Sprintf(`
data "dokploy_organization" "current" {
  name = %q
}

resource "dokploy_ssh_key" "test" {
  organization_id = data.dokploy_organization.current.id
  name            = "tf-acc-srv-key-%d"
  private_key     = %q
}

resource "dokploy_server" "test" {
  name        = "tf-acc-srv-%d"
  description = "acc test"
  ip_address  = %q
  port        = %s
  username    = %q
  ssh_key_id  = dokploy_ssh_key.test.id
  server_type = "deploy"
}`, firstOrgName(t), suffix, priv, suffix, ip, port, user)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_server.test", "id"),
					resource.TestCheckResourceAttr("dokploy_server.test", "server_type", "deploy"),
				),
			},
			{
				ResourceName:      "dokploy_server.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
```

> `dokploy_ssh_key.test` is created here without `public_key` — but we explicitly pass `private_key`. The provider generates only the *public* part to send to Dokploy. Wait — it actually generates both when both are missing. To use a known-good keypair, the test passes private_key only, and the provider derives the matching public key... **except this is not currently supported by `generateSSHKeyPair`**, which always generates a new pair. For the opt-in test to work end-to-end, the test config must supply **both** `private_key` AND `public_key` (the pair whose pub is on the VM's authorized_keys). Update the test config to also accept `DOKPLOY_TEST_SERVER_PUBLIC_KEY` and pass it as `public_key = ...`. Adjust this Step before running.

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewServerResource`.

- [ ] **Step 7: Write `internal/provider/server_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &serverResource{}
	_ resource.ResourceWithConfigure   = &serverResource{}
	_ resource.ResourceWithImportState = &serverResource{}
)

type serverResource struct {
	client *client.Client
}

func NewServerResource() resource.Resource { return &serverResource{} }

type serverModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	IPAddress      types.String `tfsdk:"ip_address"`
	Port           types.Int64  `tfsdk:"port"`
	Username       types.String `tfsdk:"username"`
	SshKeyID       types.String `tfsdk:"ssh_key_id"`
	ServerType     types.String `tfsdk:"server_type"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *serverResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (r *serverResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A remote machine registered as a managed worker in Dokploy. Creating this resource performs an SSH handshake — the corresponding public key (from `dokploy_ssh_key.x.public_key`) must already be in the remote's `~/.ssh/authorized_keys`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Description (required by the API as a present field; defaults to empty string)."},
			"ip_address":  schema.StringAttribute{Required: true, MarkdownDescription: "IP address or hostname. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"port":        schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(22), MarkdownDescription: "SSH port (default 22)."},
			"username":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("root"), MarkdownDescription: "SSH username (default `root`)."},
			"ssh_key_id":  schema.StringAttribute{Required: true, MarkdownDescription: "`dokploy_ssh_key.x.id`. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"server_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("deploy"), MarkdownDescription: "`deploy` (default — runs workloads) or `build` (used as a build host)."},
			"organization_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization the server belongs to (derived from the SSH key).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *serverResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (m serverModel) toInput() client.ServerInput {
	return client.ServerInput{
		Name:        m.Name.ValueString(),
		Description: m.Description.ValueString(),
		IPAddress:   m.IPAddress.ValueString(),
		Port:        int(m.Port.ValueInt64()),
		Username:    m.Username.ValueString(),
		SshKeyID:    m.SshKeyID.ValueString(),
		ServerType:  m.ServerType.ValueString(),
	}
}

func (r *serverResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.CreateServer(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating server (SSH handshake may have failed — verify the public key is in the remote authorized_keys)", err.Error())
		return
	}
	plan.ID = types.StringValue(s.ID)
	plan.OrganizationID = types.StringValue(s.OrganizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serverResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.GetServer(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading server", err.Error())
		return
	}
	state.Name = types.StringValue(s.Name)
	state.Description = types.StringValue(s.Description)
	state.IPAddress = types.StringValue(s.IPAddress)
	state.Port = types.Int64Value(int64(s.Port))
	state.Username = types.StringValue(s.Username)
	state.SshKeyID = types.StringValue(s.SshKeyID)
	state.ServerType = types.StringValue(s.ServerType)
	state.OrganizationID = types.StringValue(s.OrganizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *serverResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdateServer(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating server", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serverResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteServer(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting server", err.Error())
	}
}

func (r *serverResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run acceptance test if env var present**

Append `NewServerResource,` to `Resources()` in `internal/provider/provider.go`.

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run TestAccServerResource -v -timeout 10m
```

Expected: build clean. Test will SKIP unless the user has set the `DOKPLOY_TEST_SERVER_*` env vars.

- [ ] **Step 9: Commit**

```bash
git add internal/client/server.go internal/client/server_test.go \
        internal/provider/server_resource.go internal/provider/server_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: dokploy_server resource"
```

---

## Task 4: dokploy_server_schedule resource (reuses existing schedule client)

**Files:**
- Modify: `internal/client/schedule.go` (add `ServerID` to `ScheduleInput`)
- Create: `internal/provider/server_schedule_resource.go`
- Create: `internal/provider/server_schedule_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewServerScheduleResource`)

- [ ] **Step 1: Add `ServerID` to `ScheduleInput`**

Open `internal/client/schedule.go` and find the `ScheduleInput` struct. Add a `ServerID` field after `ApplicationID`:

Before:
```go
type ScheduleInput struct {
	Name           string `json:"name,omitempty"`
	CronExpression string `json:"cronExpression,omitempty"`
	Command        string `json:"command,omitempty"`
	ShellType      string `json:"shellType,omitempty"`
	ScheduleType   string `json:"scheduleType,omitempty"`
	ApplicationID  string `json:"applicationId,omitempty"`
	Enabled        *bool  `json:"enabled,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
}
```

After:
```go
type ScheduleInput struct {
	Name           string `json:"name,omitempty"`
	CronExpression string `json:"cronExpression,omitempty"`
	Command        string `json:"command,omitempty"`
	ShellType      string `json:"shellType,omitempty"`
	ScheduleType   string `json:"scheduleType,omitempty"`
	ApplicationID  string `json:"applicationId,omitempty"`
	ServerID       string `json:"serverId,omitempty"`
	Enabled        *bool  `json:"enabled,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
}
```

Verify the existing `Schedule` struct already has `ServerID` (it does — added in v0.3).

- [ ] **Step 2: Add a failing client unit test**

Append to `internal/client/schedule_test.go`:

```go
func TestCreateSchedule_Server(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body ScheduleInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ScheduleType != "server" || body.ServerID != "srv1" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Schedule{ID: "s3", Name: body.Name, ScheduleType: body.ScheduleType, ServerID: body.ServerID})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	s, err := c.CreateSchedule(context.Background(), ScheduleInput{
		Name:           "weekly-vacuum",
		CronExpression: "0 4 * * 0",
		Command:        "echo",
		ScheduleType:   "server",
		ServerID:       "srv1",
	})
	if err != nil {
		t.Fatalf("CreateSchedule() error = %v", err)
	}
	if s.ID != "s3" || s.ServerID != "srv1" {
		t.Errorf("s = %+v", s)
	}
}
```

Run: `go test ./internal/client/ -run TestCreateSchedule_Server -v` — Expected: PASS (Step 1's struct change makes it pass directly).

- [ ] **Step 3: Write the failing acceptance test (opt-in)**

`internal/provider/server_schedule_resource_test.go`:

```go
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccServerScheduleResource(t *testing.T) {
	if os.Getenv("DOKPLOY_TEST_SERVER_IP") == "" {
		t.Skip("set DOKPLOY_TEST_SERVER_IP (and friends) to run.")
	}
	ip := os.Getenv("DOKPLOY_TEST_SERVER_IP")
	user := os.Getenv("DOKPLOY_TEST_SERVER_USER")
	port := os.Getenv("DOKPLOY_TEST_SERVER_PORT")
	priv := os.Getenv("DOKPLOY_TEST_SERVER_PRIVATE_KEY")
	pub := os.Getenv("DOKPLOY_TEST_SERVER_PUBLIC_KEY")

	suffix := randInt()
	config := fmt.Sprintf(`
data "dokploy_organization" "current" {
  name = %q
}

resource "dokploy_ssh_key" "test" {
  organization_id = data.dokploy_organization.current.id
  name            = "tf-acc-ssched-key-%d"
  private_key     = %q
  public_key      = %q
}

resource "dokploy_server" "test" {
  name        = "tf-acc-ssched-srv-%d"
  ip_address  = %q
  port        = %s
  username    = %q
  ssh_key_id  = dokploy_ssh_key.test.id
}

resource "dokploy_server_schedule" "test" {
  server_id       = dokploy_server.test.id
  name            = "tf-acc-server-sched-%d"
  cron_expression = "0 5 * * *"
  command         = "echo hi"
}`, firstOrgName(t), suffix, priv, pub, suffix, ip, port, user, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_server_schedule.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_server_schedule.test", "app_name"),
				),
			},
			{
				ResourceName:      "dokploy_server_schedule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
```

- [ ] **Step 4: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewServerScheduleResource`.

- [ ] **Step 5: Write `internal/provider/server_schedule_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &serverScheduleResource{}
	_ resource.ResourceWithConfigure   = &serverScheduleResource{}
	_ resource.ResourceWithImportState = &serverScheduleResource{}
)

type serverScheduleResource struct {
	client *client.Client
}

func NewServerScheduleResource() resource.Resource { return &serverScheduleResource{} }

type serverScheduleModel struct {
	ID             types.String `tfsdk:"id"`
	ServerID       types.String `tfsdk:"server_id"`
	Name           types.String `tfsdk:"name"`
	CronExpression types.String `tfsdk:"cron_expression"`
	Command        types.String `tfsdk:"command"`
	ShellType      types.String `tfsdk:"shell_type"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Timezone       types.String `tfsdk:"timezone"`
	AppName        types.String `tfsdk:"app_name"`
}

func (r *serverScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_schedule"
}

func (r *serverScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A cron-scheduled command run on a managed Dokploy server (`scheduleType: server`).",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"server_id":       schema.StringAttribute{Required: true, MarkdownDescription: "`dokploy_server.x.id`. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":            schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"cron_expression": schema.StringAttribute{Required: true, MarkdownDescription: "Cron expression."},
			"command":         schema.StringAttribute{Required: true, MarkdownDescription: "Shell command to run."},
			"shell_type":      schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Shell. Defaults to `bash`.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"enabled":         schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether the schedule is active. Defaults to `true`."},
			"timezone":        schema.StringAttribute{Optional: true, MarkdownDescription: "IANA timezone."},
			"app_name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Internal name generated by Dokploy.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *serverScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (m serverScheduleModel) toInput() client.ScheduleInput {
	in := client.ScheduleInput{
		Name:           m.Name.ValueString(),
		CronExpression: m.CronExpression.ValueString(),
		Command:        m.Command.ValueString(),
		ScheduleType:   "server",
		ServerID:       m.ServerID.ValueString(),
		ShellType:      m.ShellType.ValueString(),
		Timezone:       m.Timezone.ValueString(),
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		v := m.Enabled.ValueBool()
		in.Enabled = &v
	}
	return in
}

func (r *serverScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.CreateSchedule(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating server schedule", err.Error())
		return
	}
	plan.ID = types.StringValue(s.ID)
	plan.AppName = types.StringValue(s.AppName)
	plan.ShellType = types.StringValue(s.ShellType)
	plan.Enabled = types.BoolValue(s.Enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serverScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverScheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.GetSchedule(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading schedule", err.Error())
		return
	}
	state.ServerID = types.StringValue(s.ServerID)
	state.Name = types.StringValue(s.Name)
	state.CronExpression = types.StringValue(s.CronExpression)
	state.Command = types.StringValue(s.Command)
	state.ShellType = types.StringValue(s.ShellType)
	state.Enabled = types.BoolValue(s.Enabled)
	if s.Timezone != "" || !state.Timezone.IsNull() {
		state.Timezone = types.StringValue(s.Timezone)
	}
	state.AppName = types.StringValue(s.AppName)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *serverScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdateSchedule(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating server schedule", err.Error())
		return
	}
	// Re-read to populate computed fields after update.
	s, err := r.client.GetSchedule(ctx, plan.ID.ValueString())
	if err == nil {
		plan.ShellType = types.StringValue(s.ShellType)
		plan.Enabled = types.BoolValue(s.Enabled)
		plan.AppName = types.StringValue(s.AppName)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serverScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverScheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSchedule(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting schedule", err.Error())
	}
}

func (r *serverScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 6: Register, build, run unit + opt-in acceptance**

Append `NewServerScheduleResource,` to `Resources()`.

```bash
gofmt -w .
go build ./...
go test ./internal/client/... -v
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run TestAccServerScheduleResource -v -timeout 10m
```

Expected: build clean; unit tests PASS; acceptance test SKIPs without env vars set.

- [ ] **Step 7: Commit**

```bash
git add internal/client/schedule.go internal/client/schedule_test.go \
        internal/provider/server_schedule_resource.go internal/provider/server_schedule_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: dokploy_server_schedule resource"
```

---

## Task 5: Add server_id to existing 6 resources

This task touches 12 files mechanically: 6 client files (add `ServerID` to the response struct and the input struct) and 6 resource files (add `server_id` schema attribute, model field, and propagation through Create/Read).

For brevity, the patterns below show the application + postgres modifications in full code. The remaining 4 (`mysql`, `mariadb`, `mongo`, `redis`) are byte-identical to the postgres pattern — apply the same diff to each.

**Files:**
- Modify: `internal/client/application.go`, `internal/client/postgres.go`, `internal/client/mysql.go`, `internal/client/mariadb.go`, `internal/client/mongo.go`, `internal/client/redis.go`
- Modify: `internal/provider/application_resource.go`, `internal/provider/postgres_resource.go`, `internal/provider/mysql_resource.go`, `internal/provider/mariadb_resource.go`, `internal/provider/mongo_resource.go`, `internal/provider/redis_resource.go`

- [ ] **Step 1: Add `ServerID` to all 6 client structs and inputs**

For each of the 6 client files, add `ServerID *string` to the resource struct and to the `*Input` struct. Apply this diff to **each** of `application.go`, `postgres.go`, `mysql.go`, `mariadb.go`, `mongo.go`, `redis.go`:

In the resource struct (e.g. `Application`, `Postgres`, …), add right before the closing brace:

```go
	ServerID *string `json:"serverId"`
```

In the `*Input` struct (e.g. `ApplicationInput`, `PostgresInput`, …), add right before the closing brace:

```go
	ServerID *string `json:"serverId,omitempty"`
```

> Note: the `omitempty` on the input matters — when the user doesn't set `server_id`, we don't want to send `"serverId":null` (which the API may interpret differently from omitting the field). The response struct has no `omitempty` so it can deserialize an explicit null.

Run: `go test ./internal/client/... -v`
Expected: PASS (additive struct change does not affect existing tests).

- [ ] **Step 2: Add a failing unit test that verifies the new field**

Append to `internal/client/application_test.go`:

```go
func TestCreateApplication_WithServerID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body ApplicationInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ServerID == nil || *body.ServerID != "srv1" {
			t.Errorf("serverId not sent: body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Application{ID: "app1", Name: body.Name})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	srvID := "srv1"
	_, err := c.CreateApplication(context.Background(), ApplicationInput{
		Name:          "api",
		AppName:       "api",
		EnvironmentID: "env",
		ServerID:      &srvID,
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
}
```

Run: `go test ./internal/client/ -run TestCreateApplication_WithServerID -v` — Expected: PASS (the struct already has `ServerID` after Step 1).

- [ ] **Step 3: Add `server_id` attribute to `dokploy_application` schema and plumb through Create/Read**

In `internal/provider/application_resource.go`:

Find the `applicationModel` struct. Add right before the closing brace:

```go
	ServerID types.String `tfsdk:"server_id"`
```

Find the `Schema` method and add this entry to the `Attributes` map, right after `"app_name"`:

```go
			"server_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Managed server (`dokploy_server.x.id`) the application runs on. Omit to run on the Dokploy host. Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
```

Find the `Create` method's call to `r.client.CreateApplication`. Add `ServerID: optionalString(plan.ServerID),` to the struct literal — right after `EnvironmentID`:

```go
	app, err := r.client.CreateApplication(ctx, client.ApplicationInput{
		Name:          plan.Name.ValueString(),
		AppName:       slugify(plan.Name.ValueString()),
		Description:   plan.Description.ValueString(),
		EnvironmentID: plan.EnvironmentID.ValueString(),
		ServerID:      optionalString(plan.ServerID),
	})
```

In the `Read` method, after the other field mappings, add:

```go
	if app.ServerID != nil {
		state.ServerID = types.StringValue(*app.ServerID)
	} else {
		state.ServerID = types.StringNull()
	}
```

> `optionalString` already exists in `internal/provider/application_resource.go` from v0.1 (returns `*string` from `types.String`, nil when null/unknown/empty). It's package-level so all resource files in the `provider` package can call it. Verify with `grep -n 'func optionalString' internal/provider/`. If the function lives in a file other than `database_helpers.go`, no need to move it — leave it where it is.

- [ ] **Step 4: Repeat Step 3 for each of the 5 database resources**

The 5 database resources (`postgres_resource.go`, `mysql_resource.go`, `mariadb_resource.go`, `mongo_resource.go`, `redis_resource.go`) follow exactly the same pattern. Apply each of these three changes to every one:

1. Add `ServerID types.String \`tfsdk:"server_id"\`` to the model struct.
2. Add the same `"server_id"` schema attribute (the snippet above) to the `Attributes` map.
3. In the `Create` method, when building the `client.<Type>Input{...}` struct, add `ServerID: optionalString(plan.ServerID),` to the literal.
4. In the `Read` method, add the same `if app.ServerID != nil { ... } else { ... }` block (substitute `app` with the local variable name used by each resource — `pg`, `my`, `ma`, `mo`, `re`).

After all 6 are done, run:

```bash
go build ./...
gofmt -w .
go vet ./...
```

Expected: build/vet clean.

- [ ] **Step 5: Acceptance tests (opt-in)**

Append to `internal/provider/application_resource_test.go`:

```go
func TestAccApplicationResource_OnServer(t *testing.T) {
	if os.Getenv("DOKPLOY_TEST_SERVER_IP") == "" {
		t.Skip("set DOKPLOY_TEST_SERVER_IP (and friends) to run.")
	}
	ip := os.Getenv("DOKPLOY_TEST_SERVER_IP")
	user := os.Getenv("DOKPLOY_TEST_SERVER_USER")
	port := os.Getenv("DOKPLOY_TEST_SERVER_PORT")
	priv := os.Getenv("DOKPLOY_TEST_SERVER_PRIVATE_KEY")
	pub := os.Getenv("DOKPLOY_TEST_SERVER_PUBLIC_KEY")
	suffix := randInt()
	config := fmt.Sprintf(`
data "dokploy_organization" "current" { name = %q }
resource "dokploy_ssh_key" "test" {
  organization_id = data.dokploy_organization.current.id
  name            = "tf-acc-app-srv-key-%d"
  private_key     = %q
  public_key      = %q
}
resource "dokploy_server" "test" {
  name        = "tf-acc-app-srv-%d"
  ip_address  = %q
  port        = %s
  username    = %q
  ssh_key_id  = dokploy_ssh_key.test.id
}
resource "dokploy_project" "test" {
  name = "tf-acc-app-srv-proj-%d"
}
resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-app-on-server"
  docker_image   = "nginx:1.27"
  server_id      = dokploy_server.test.id
  timeouts { create = "15m" update = "15m" }
}`, firstOrgName(t), suffix, priv, pub, suffix, ip, port, user, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_application.test", "id"),
					resource.TestCheckResourceAttrPair("dokploy_application.test", "server_id", "dokploy_server.test", "id"),
				),
			},
		},
	})
}
```

Add the `"os"` import to the test file if not already present.

Append the equivalent test to `postgres_resource_test.go`:

```go
func TestAccPostgresResource_OnServer(t *testing.T) {
	if os.Getenv("DOKPLOY_TEST_SERVER_IP") == "" {
		t.Skip("set DOKPLOY_TEST_SERVER_IP (and friends) to run.")
	}
	ip := os.Getenv("DOKPLOY_TEST_SERVER_IP")
	user := os.Getenv("DOKPLOY_TEST_SERVER_USER")
	port := os.Getenv("DOKPLOY_TEST_SERVER_PORT")
	priv := os.Getenv("DOKPLOY_TEST_SERVER_PRIVATE_KEY")
	pub := os.Getenv("DOKPLOY_TEST_SERVER_PUBLIC_KEY")
	suffix := randInt()
	config := fmt.Sprintf(`
data "dokploy_organization" "current" { name = %q }
resource "dokploy_ssh_key" "test" {
  organization_id = data.dokploy_organization.current.id
  name            = "tf-acc-pg-srv-key-%d"
  private_key     = %q
  public_key      = %q
}
resource "dokploy_server" "test" {
  name        = "tf-acc-pg-srv-%d"
  ip_address  = %q
  port        = %s
  username    = %q
  ssh_key_id  = dokploy_ssh_key.test.id
}
resource "dokploy_project" "test" {
  name = "tf-acc-pg-srv-proj-%d"
}
resource "dokploy_postgres" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-pg-on-server"
  docker_image   = "postgres:16"
  database_name  = "app"
  database_user  = "app"
  server_id      = dokploy_server.test.id
  timeouts { create = "15m" update = "15m" }
}`, firstOrgName(t), suffix, priv, pub, suffix, ip, port, user, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_postgres.test", "id"),
					resource.TestCheckResourceAttrPair("dokploy_postgres.test", "server_id", "dokploy_server.test", "id"),
				),
			},
		},
	})
}
```

Run the existing v0.1+v0.2 acceptance suite to confirm no regression (these tests don't set `server_id`, so the new attribute defaults to null and the resources run on the Dokploy host as before):

```bash
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run "TestAccApplicationResource$|TestAccPostgresResource$|TestAccMysqlResource$|TestAccMariadbResource$|TestAccMongoResource$|TestAccRedisResource$" -v -timeout 30m
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w .
git add internal/client/application.go internal/client/postgres.go internal/client/mysql.go \
        internal/client/mariadb.go internal/client/mongo.go internal/client/redis.go \
        internal/client/application_test.go \
        internal/provider/application_resource.go internal/provider/postgres_resource.go \
        internal/provider/mysql_resource.go internal/provider/mariadb_resource.go \
        internal/provider/mongo_resource.go internal/provider/redis_resource.go \
        internal/provider/application_resource_test.go internal/provider/postgres_resource_test.go \
        internal/provider/database_helpers.go
git commit -m "feat: server_id attribute on application + 5 databases"
```

---

## Task 6: Examples, README, generated docs

**Files:**
- Create: `examples/resources/dokploy_ssh_key/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_server/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_server_schedule/resource.tf` + `import.sh`
- Modify: `README.md`
- Regenerated: `docs/resources/*.md` (via `go generate ./...`)

- [ ] **Step 1: Create example files**

`examples/resources/dokploy_ssh_key/resource.tf`:

```hcl
data "dokploy_organization" "current" {
  name = "My Organization"
}

resource "dokploy_ssh_key" "worker" {
  organization_id = data.dokploy_organization.current.id
  name            = "worker-key"
  # private_key/public_key omitted → provider generates 4096-bit RSA.
}

output "worker_public_key" {
  value = dokploy_ssh_key.worker.public_key
  # Add this string to the remote VM's ~/.ssh/authorized_keys before creating a dokploy_server.
}
```

`examples/resources/dokploy_ssh_key/import.sh`:

```bash
terraform import dokploy_ssh_key.worker <sshKeyId>
```

`examples/resources/dokploy_server/resource.tf`:

```hcl
resource "dokploy_server" "worker_sp" {
  name        = "worker-sp"
  description = "Worker São Paulo"
  ip_address  = "203.0.113.10"
  port        = 22
  username    = "dokploy"
  ssh_key_id  = dokploy_ssh_key.worker.id
  server_type = "deploy"
}
```

`examples/resources/dokploy_server/import.sh`:

```bash
terraform import dokploy_server.worker_sp <serverId>
```

`examples/resources/dokploy_server_schedule/resource.tf`:

```hcl
resource "dokploy_server_schedule" "vacuum" {
  server_id       = dokploy_server.worker_sp.id
  name            = "pg-vacuum-weekly"
  cron_expression = "0 4 * * 0"
  command         = "docker exec ${dokploy_postgres.db.app_name} psql -U app -c 'VACUUM ANALYZE'"
  timezone        = "America/Sao_Paulo"
}
```

`examples/resources/dokploy_server_schedule/import.sh`:

```bash
terraform import dokploy_server_schedule.vacuum <scheduleId>
```

- [ ] **Step 2: Update `README.md`**

Find the line `- \`dokploy_host_schedule\` — cron command on the Dokploy host` (last line of the v0.3 resources block in the `## Resources` section). Insert these three lines directly after it, before `## Data sources`:

```markdown
- `dokploy_ssh_key` — SSH key registered at the organization level (used by `dokploy_server`)
- `dokploy_server` — remote machine registered as a managed worker
- `dokploy_server_schedule` — cron command on a managed server
```

- [ ] **Step 3: Regenerate documentation**

Run: `go generate ./...`
Expected: three new files under `docs/resources/`: `ssh_key.md`, `server.md`, `server_schedule.md`. Existing resource docs may regenerate (they should now reflect the new `server_id` attribute in application/postgres/mysql/mariadb/mongo/redis).

- [ ] **Step 4: Verify**

```bash
git status --short
go build ./...
go vet ./...
```

Expected: build/vet clean. Diff shows new examples, README change, and modified+new docs files. `docs/superpowers/` untouched.

- [ ] **Step 5: Commit**

```bash
gofmt -w .
git add examples README.md docs/
git commit -m "docs: examples, README entries, and generated docs for v0.4 servers and ssh keys"
```

---

## Task 7: Release v0.4.0

**Files:** none (release lives in tags + GoReleaser-built GitHub Release).

- [ ] **Step 1: Final verification**

```bash
cd /Users/lukearch/Projects/My/dokploy-terraform-provider
go build ./...
go vet ./...
go test ./internal/client/... -v
go generate ./...
git status --short
```

Expected: all green; no uncommitted docs diff.

Run the acceptance suite (without server-dependent opt-ins by default):

```bash
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/... -v -timeout 90m
```

Expected: every test PASS (server-dependent ones SKIP if you haven't set the `DOKPLOY_TEST_SERVER_*` env vars). Confirm the live instance has no `tf-acc-*` resources left.

- [ ] **Step 2: Merge to master**

If on a feature branch:

```bash
git checkout master
git pull --ff-only
git merge --ff-only <feature-branch>
git push origin master
```

- [ ] **Step 3: Tag and push v0.4.0**

```bash
git tag v0.4.0
git push origin v0.4.0
```

The Release workflow triggers automatically.

- [ ] **Step 4: Watch the workflow**

```bash
RUN_ID=$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')
gh run watch "$RUN_ID" --exit-status
```

Expected: SUCCESS in ~4 minutes.

- [ ] **Step 5: Verify the GitHub Release**

```bash
gh release view v0.4.0 --json assets -q '.assets[].name' | sort
gh release download v0.4.0 -p '*_SHA256SUMS' -O - | grep manifest
```

Expected: 13 assets named `0.4.0`, and the `SHA256SUMS` line containing `0.4.0_manifest.json`.

- [ ] **Step 6: Confirm Registry picked up v0.4.0**

```bash
/usr/bin/curl -s "https://registry.terraform.io/v1/providers/lucasaarch/dokploy/versions" \
  | /opt/homebrew/bin/python3 -m json.tool | grep -E '"version"' | head -5
```

Expected: a `"version": "0.4.0"` entry. May take ~5 min after the workflow finishes.

---

## Self-review checklist

Before considering this plan complete, the implementer should confirm:

- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` clean
- [ ] `go test ./internal/client/... -v` passes
- [ ] `go generate ./...` produces no uncommitted diff
- [ ] `TestAccSshKeyResource` passes against the live instance
- [ ] All v0.1 + v0.2 + v0.3 acceptance tests still pass (no regression — the new `server_id` attribute defaults to null when omitted)
- [ ] Live instance clean of `tf-acc-*` resources after the suite
- [ ] All three new resources registered in `internal/provider/provider.go`'s `Resources()` list
- [ ] `ServerID` field added to all six client structs and their `*Input` types
- [ ] `v0.4.0` tag pushed and GitHub Release published with 13 signed assets
- [ ] Registry shows `0.4.0`
