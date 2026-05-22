# Dokploy Terraform Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Terraform provider for Dokploy with 5 resources (organization, project, environment, application, domain) that automates a full Docker-based deployment graph.

**Architecture:** Two layers. `internal/client` is a thin, Terraform-agnostic HTTP client for the Dokploy RPC-style API, fully unit-tested with `httptest`. `internal/provider` implements `terraform-plugin-framework` resources that translate Terraform state to/from the client. The `dokploy_application` resource orchestrates create + configure + deploy + status-polling in a single operation.

**Tech Stack:** Go 1.26, `terraform-plugin-framework`, `terraform-plugin-framework-timeouts`, `terraform-plugin-testing`, GoReleaser, GitHub Actions, `terraform-plugin-docs`.

**Spec:** `docs/superpowers/specs/2026-05-22-dokploy-terraform-provider-design.md`

---

## Conventions for every task

- TDD: write the failing test first, see it fail, implement, see it pass, commit.
- Run `gofmt -w .` before every commit.
- Commit messages: conventional commits (`feat:`, `test:`, `chore:`, `docs:`).
- Unit tests need no network. Acceptance tests (`TestAcc*`) run only when `TF_ACC=1` and require env vars `DOKPLOY_ENDPOINT` and `DOKPLOY_API_KEY`.

---

## Task 1: Verify the live Dokploy API and write the API reference

This task is exploratory, not TDD. It produces a reference document that every later client task depends on. It requires the user's live instance — ask the user for `DOKPLOY_ENDPOINT` and `DOKPLOY_API_KEY` before starting.

**Files:**
- Create: `internal/client/API.md`

- [ ] **Step 1: Fetch the OpenAPI/Swagger spec**

Run (substitute the real endpoint):

```bash
curl -s "$DOKPLOY_ENDPOINT/swagger/json" -o /tmp/dokploy-swagger.json \
  || curl -s "$DOKPLOY_ENDPOINT/api/swagger.json" -o /tmp/dokploy-swagger.json
python3 -m json.tool /tmp/dokploy-swagger.json | head -50
```

If neither path works, open `$DOKPLOY_ENDPOINT/swagger` in a browser and locate the JSON spec URL from the network tab.

- [ ] **Step 2: Extract the relevant endpoints**

For each of these routers, record the exact HTTP method, path, request body fields, and response body fields:

- `organization.*` — create, one/get, update, remove/delete, all
- `project.*` — create, one, update, remove, all
- `environment.*` — create, one, update, remove, byProjectId/all
- `application.*` — create, one, update, delete, saveDockerProvider, saveEnvironment, deploy
- `domain.*` — create, one/byApplicationId, update, delete

Run, for example:

```bash
python3 -c "import json; d=json.load(open('/tmp/dokploy-swagger.json')); [print(m.upper(), p) for p,ms in d['paths'].items() for m in ms if any(p.startswith('/'+r) or ('/'+r) in p for r in ['organization','project','environment','application','domain'])]"
```

- [ ] **Step 3: Confirm the deployment status field**

Create a throwaway application via the UI, call `application.one`, and record which field carries deploy status and its possible values (expected: `applicationStatus` with `idle`/`running`/`done`/`error` — confirm exact strings).

- [ ] **Step 4: Confirm three risk items from the spec**

Record answers in `API.md`:
1. Does `application.one` return `registryPassword` / docker credentials, or omit them?
2. Does `organization.remove` succeed on a normal org, or error on the active/last org?
3. What field on `project.create` / `project.one` carries the organization id, and is it required on create?

- [ ] **Step 5: Write `internal/client/API.md`**

Document, for every endpoint above: HTTP method, full path (relative to `/api`), request JSON shape, response JSON shape. This file is the source of truth for Tasks 3–8. Where this plan's code differs from `API.md`, `API.md` wins — adjust field names/paths accordingly.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; git add internal/client/API.md && git commit -m "docs: dokploy API reference from live swagger"
```

---

## Task 2: Project scaffolding

**Files:**
- Modify: `go.mod`
- Create: `main.go`
- Create: `internal/provider/provider.go`
- Create: `Makefile`
- Create: `.golangci.yml`
- Create: `tools/tools.go`

- [ ] **Step 1: Add dependencies**

Run:

```bash
go get github.com/hashicorp/terraform-plugin-framework@latest
go get github.com/hashicorp/terraform-plugin-framework-timeouts@latest
go get github.com/hashicorp/terraform-plugin-framework-validators@latest
go get github.com/hashicorp/terraform-plugin-log@latest
go get github.com/hashicorp/terraform-plugin-testing@latest
```

- [ ] **Step 2: Create `main.go`**

```go
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/provider"
)

// version is set at build time by GoReleaser via -ldflags.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with debugger support")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/lucasaarch/dokploy",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
```

- [ ] **Step 3: Create `internal/provider/provider.go` skeleton**

The `Configure` body and resource list are filled in Task 9. For now it must compile.

```go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure dokployProvider satisfies the provider.Provider interface.
var _ provider.Provider = &dokployProvider{}

type dokployProvider struct {
	version string
}

// New returns a provider factory used by main.go and acceptance tests.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &dokployProvider{version: version}
	}
}

// dokployProviderModel maps the provider configuration block.
type dokployProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func (p *dokployProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "dokploy"
	resp.Version = p.version
}

func (p *dokployProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Dokploy provider manages resources on a Dokploy instance.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Base URL of the Dokploy instance, e.g. `https://dokploy.example.com`. May also be set via the `DOKPLOY_ENDPOINT` environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Dokploy API key sent as the `x-api-key` header. May also be set via the `DOKPLOY_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *dokployProvider) Configure(_ context.Context, _ provider.ConfigureRequest, _ *provider.ConfigureResponse) {
	// Implemented in Task 9.
}

func (p *dokployProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil // Populated in Task 9.
}

func (p *dokployProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
```

- [ ] **Step 4: Create `Makefile`**

```makefile
default: build

build:
	go build -o terraform-provider-dokploy

test:
	go test ./internal/client/... -v

testacc:
	TF_ACC=1 go test ./internal/provider/... -v -timeout 30m

lint:
	golangci-lint run

fmt:
	gofmt -w .

docs:
	go generate ./...

.PHONY: default build test testacc lint fmt docs
```

- [ ] **Step 5: Create `.golangci.yml`**

```yaml
run:
  timeout: 5m
linters:
  enable:
    - gofmt
    - govet
    - ineffassign
    - staticcheck
    - unused
    - errcheck
```

- [ ] **Step 6: Create `tools/tools.go`** (pins the docs generator)

```go
//go:build tools

package tools

import (
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
```

Run: `go get github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest`

- [ ] **Step 7: Verify it builds**

Run: `go mod tidy && go build ./...`
Expected: no errors, `terraform-provider-dokploy` is buildable.

- [ ] **Step 8: Commit**

```bash
gofmt -w . ; git add -A && git commit -m "chore: scaffold provider project structure"
```

---

## Task 3: API client core (`client.go`)

**Files:**
- Create: `internal/client/client.go`
- Test: `internal/client/client_test.go`

- [ ] **Step 1: Write the failing test**

`internal/client/client_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDo_GETSendsAPIKeyAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "secret" {
			t.Errorf("x-api-key = %q, want %q", got, "secret")
		}
		if r.URL.Path != "/api/project.one" {
			t.Errorf("path = %q, want /api/project.one", r.URL.Path)
		}
		if got := r.URL.Query().Get("projectId"); got != "p1" {
			t.Errorf("query projectId = %q, want p1", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "demo"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	var out struct {
		Name string `json:"name"`
	}
	q := url.Values{"projectId": {"p1"}}
	if err := c.do(context.Background(), http.MethodGet, "project.one", nil, q, &out); err != nil {
		t.Fatalf("do() error = %v", err)
	}
	if out.Name != "demo" {
		t.Errorf("out.Name = %q, want demo", out.Name)
	}
}

func TestDo_ErrorStatusReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	err := c.do(context.Background(), http.MethodGet, "project.one", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false, want true for err %v", err)
	}
}

func TestDo_POSTSendsJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "demo" {
			t.Errorf("body name = %q, want demo", body["name"])
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"projectId": "p1"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	var out struct {
		ProjectID string `json:"projectId"`
	}
	in := map[string]string{"name": "demo"}
	if err := c.do(context.Background(), http.MethodPost, "project.create", in, nil, &out); err != nil {
		t.Fatalf("do() error = %v", err)
	}
	if out.ProjectID != "p1" {
		t.Errorf("out.ProjectID = %q, want p1", out.ProjectID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -run TestDo -v`
Expected: FAIL — `undefined: New`, `undefined: IsNotFound`.

- [ ] **Step 3: Write `internal/client/client.go`**

```go
// Package client is a thin, Terraform-agnostic HTTP client for the Dokploy API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a single Dokploy instance.
type Client struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

// New builds a Client. endpoint is the instance base URL (no trailing /api).
func New(endpoint, apiKey string) *Client {
	return &Client{
		endpoint:   strings.TrimRight(endpoint, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// APIError is returned for any HTTP status >= 400.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("dokploy API error (status %d): %s", e.StatusCode, e.Message)
}

// IsNotFound reports whether err is an APIError with HTTP 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// do performs an HTTP request against /api/<path>. body is JSON-encoded when
// non-nil; out is JSON-decoded from the response when non-nil.
func (c *Client) do(ctx context.Context, method, path string, body any, query url.Values, out any) error {
	target := c.endpoint + "/api/" + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &APIError{StatusCode: resp.StatusCode, Message: parseErrorMessage(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decoding response body: %w", err)
		}
	}
	return nil
}

// parseErrorMessage extracts a human-readable message from an error response.
func parseErrorMessage(body []byte) string {
	var parsed struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.Message != "" {
			return parsed.Message
		}
		if parsed.Error != "" {
			return parsed.Error
		}
	}
	return strings.TrimSpace(string(body))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/client/ -run TestDo -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/client/client.go internal/client/client_test.go && git commit -m "feat: dokploy API client core with typed errors"
```

---

## Task 4: Organization client methods

Adjust JSON field names/paths to match `internal/client/API.md` from Task 1 if they differ.

**Files:**
- Create: `internal/client/organization.go`
- Test: `internal/client/organization_test.go`

- [ ] **Step 1: Write the failing test**

`internal/client/organization_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateOrganization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/organization.create" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "acme" {
			t.Errorf("name = %q, want acme", body["name"])
		}
		_ = json.NewEncoder(w).Encode(Organization{ID: "org1", Name: "acme"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	org, err := c.CreateOrganization(context.Background(), OrganizationInput{Name: "acme"})
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if org.ID != "org1" || org.Name != "acme" {
		t.Errorf("org = %+v", org)
	}
}

func TestGetOrganization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("organizationId") != "org1" {
			t.Errorf("organizationId = %q", r.URL.Query().Get("organizationId"))
		}
		_ = json.NewEncoder(w).Encode(Organization{ID: "org1", Name: "acme"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	org, err := c.GetOrganization(context.Background(), "org1")
	if err != nil {
		t.Fatalf("GetOrganization() error = %v", err)
	}
	if org.Name != "acme" {
		t.Errorf("org.Name = %q, want acme", org.Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -run Organization -v`
Expected: FAIL — `undefined: Organization`, `undefined: OrganizationInput`.

- [ ] **Step 3: Write `internal/client/organization.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Organization is the top-level tenancy object in Dokploy.
type Organization struct {
	ID   string `json:"organizationId"`
	Name string `json:"name"`
}

// OrganizationInput is the writable payload for create/update.
type OrganizationInput struct {
	Name string `json:"name"`
}

func (c *Client) CreateOrganization(ctx context.Context, in OrganizationInput) (*Organization, error) {
	var out Organization
	if err := c.do(ctx, http.MethodPost, "organization.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	var out Organization
	q := url.Values{"organizationId": {id}}
	if err := c.do(ctx, http.MethodGet, "organization.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateOrganization(ctx context.Context, id string, in OrganizationInput) (*Organization, error) {
	payload := struct {
		OrganizationInput
		ID string `json:"organizationId"`
	}{OrganizationInput: in, ID: id}
	var out Organization
	if err := c.do(ctx, http.MethodPost, "organization.update", payload, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteOrganization(ctx context.Context, id string) error {
	payload := map[string]string{"organizationId": id}
	return c.do(ctx, http.MethodPost, "organization.remove", payload, nil, nil)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/client/ -run Organization -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/client/organization*.go && git commit -m "feat: organization API client methods"
```

---

## Task 5: Project client methods

**Files:**
- Create: `internal/client/project.go`
- Test: `internal/client/project_test.go`

- [ ] **Step 1: Write the failing test**

`internal/client/project_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/project.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body ProjectInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "web" || body.OrganizationID != "org1" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Project{
			ID: "p1", Name: "web", OrganizationID: "org1",
			Environments: []Environment{{ID: "env-prod", Name: "production"}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	p, err := c.CreateProject(context.Background(), ProjectInput{Name: "web", OrganizationID: "org1"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if p.ID != "p1" || p.ProductionEnvironmentID() != "env-prod" {
		t.Errorf("project = %+v", p)
	}
}

func TestGetProject_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	_, err := c.GetProject(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false, want true (err = %v)", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -run Project -v`
Expected: FAIL — `undefined: Project`, `undefined: ProjectInput`.

- [ ] **Step 3: Write `internal/client/project.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Project groups environments and applications. Dokploy auto-creates a
// "production" environment on project creation.
type Project struct {
	ID             string        `json:"projectId"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	OrganizationID string        `json:"organizationId"`
	Environments   []Environment `json:"environments"`
}

// ProductionEnvironmentID returns the id of the auto-created production
// environment, or "" if not present.
func (p *Project) ProductionEnvironmentID() string {
	for _, e := range p.Environments {
		if e.Name == "production" {
			return e.ID
		}
	}
	return ""
}

// ProjectInput is the writable payload for create/update.
type ProjectInput struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
}

func (c *Client) CreateProject(ctx context.Context, in ProjectInput) (*Project, error) {
	var out Project
	if err := c.do(ctx, http.MethodPost, "project.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var out Project
	q := url.Values{"projectId": {id}}
	if err := c.do(ctx, http.MethodGet, "project.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateProject(ctx context.Context, id string, in ProjectInput) (*Project, error) {
	payload := struct {
		ProjectInput
		ID string `json:"projectId"`
	}{ProjectInput: in, ID: id}
	var out Project
	if err := c.do(ctx, http.MethodPost, "project.update", payload, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteProject(ctx context.Context, id string) error {
	payload := map[string]string{"projectId": id}
	return c.do(ctx, http.MethodPost, "project.remove", payload, nil, nil)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/client/ -run Project -v`
Expected: PASS. (`Environment` is defined in Task 6; if it does not compile yet, do Task 6 Step 3 first — define the `Environment` struct — then return here. Recommended: implement Tasks 5 and 6 together.)

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/client/project*.go && git commit -m "feat: project API client methods"
```

---

## Task 6: Environment client methods

**Files:**
- Create: `internal/client/environment.go`
- Test: `internal/client/environment_test.go`

- [ ] **Step 1: Write the failing test**

`internal/client/environment_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateEnvironment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environment.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body EnvironmentInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "staging" || body.ProjectID != "p1" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Environment{ID: "env1", Name: "staging", ProjectID: "p1"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	e, err := c.CreateEnvironment(context.Background(), EnvironmentInput{
		Name: "staging", ProjectID: "p1", Env: "LOG_LEVEL=debug",
	})
	if err != nil {
		t.Fatalf("CreateEnvironment() error = %v", err)
	}
	if e.ID != "env1" {
		t.Errorf("env.ID = %q, want env1", e.ID)
	}
}

func TestGetEnvironment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("environmentId") != "env1" {
			t.Errorf("environmentId = %q", r.URL.Query().Get("environmentId"))
		}
		_ = json.NewEncoder(w).Encode(Environment{ID: "env1", Name: "staging", Env: "LOG_LEVEL=debug"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	e, err := c.GetEnvironment(context.Background(), "env1")
	if err != nil {
		t.Fatalf("GetEnvironment() error = %v", err)
	}
	if e.Env != "LOG_LEVEL=debug" {
		t.Errorf("env.Env = %q", e.Env)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -run Environment -v`
Expected: FAIL — `undefined: Environment`, `undefined: EnvironmentInput`.

- [ ] **Step 3: Write `internal/client/environment.go`**

The `Env` field is the raw dotenv-format string Dokploy stores. Conversion to/from a Terraform `map[string]string` happens in the provider layer (Task 12), via the helper added in Task 7.

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Environment is a deployment environment inside a project.
type Environment struct {
	ID          string `json:"environmentId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ProjectID   string `json:"projectId"`
	// Env holds shared variables in dotenv format ("KEY=value\nKEY2=value2").
	Env string `json:"env"`
}

// EnvironmentInput is the writable payload for create/update.
type EnvironmentInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	Env         string `json:"env,omitempty"`
}

func (c *Client) CreateEnvironment(ctx context.Context, in EnvironmentInput) (*Environment, error) {
	var out Environment
	if err := c.do(ctx, http.MethodPost, "environment.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	var out Environment
	q := url.Values{"environmentId": {id}}
	if err := c.do(ctx, http.MethodGet, "environment.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateEnvironment(ctx context.Context, id string, in EnvironmentInput) (*Environment, error) {
	payload := struct {
		EnvironmentInput
		ID string `json:"environmentId"`
	}{EnvironmentInput: in, ID: id}
	var out Environment
	if err := c.do(ctx, http.MethodPost, "environment.update", payload, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteEnvironment(ctx context.Context, id string) error {
	payload := map[string]string{"environmentId": id}
	return c.do(ctx, http.MethodPost, "environment.remove", payload, nil, nil)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/client/ -run "Project|Environment" -v`
Expected: PASS (project and environment tests both compile and pass now).

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/client/environment*.go && git commit -m "feat: environment API client methods"
```

---

## Task 7: Dotenv helper

The provider exposes env vars as a Terraform `map(string)`; Dokploy stores them as a dotenv string. This shared helper converts between the two and is used by Tasks 12 and 13.

**Files:**
- Create: `internal/client/env.go`
- Test: `internal/client/env_test.go`

- [ ] **Step 1: Write the failing test**

`internal/client/env_test.go`:

```go
package client

import (
	"reflect"
	"testing"
)

func TestMapToDotenv_SortedAndStable(t *testing.T) {
	got := MapToDotenv(map[string]string{"B": "2", "A": "1"})
	if got != "A=1\nB=2" {
		t.Errorf("MapToDotenv() = %q, want %q", got, "A=1\nB=2")
	}
}

func TestDotenvToMap(t *testing.T) {
	got := DotenvToMap("A=1\nB=hello world\n\n# comment\nC=")
	want := map[string]string{"A": "1", "B": "hello world", "C": ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DotenvToMap() = %#v, want %#v", got, want)
	}
}

func TestMapToDotenv_Empty(t *testing.T) {
	if got := MapToDotenv(nil); got != "" {
		t.Errorf("MapToDotenv(nil) = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -run Dotenv -v`
Expected: FAIL — `undefined: MapToDotenv`, `undefined: DotenvToMap`.

- [ ] **Step 3: Write `internal/client/env.go`**

```go
package client

import (
	"sort"
	"strings"
)

// MapToDotenv renders a map as a newline-separated KEY=value string with keys
// sorted, so the output is stable regardless of map iteration order.
func MapToDotenv(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+m[k])
	}
	return strings.Join(lines, "\n")
}

// DotenvToMap parses a dotenv string into a map. Blank lines and lines
// starting with '#' are ignored.
func DotenvToMap(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/client/ -run Dotenv -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/client/env*.go && git commit -m "feat: dotenv map conversion helper"
```

---

## Task 8: Application and domain client methods

**Files:**
- Create: `internal/client/application.go`
- Create: `internal/client/domain.go`
- Test: `internal/client/application_test.go`
- Test: `internal/client/domain_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/client/application_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreateApplication(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/application.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Application{ID: "app1", Name: "api", AppName: "api-abc123"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	app, err := c.CreateApplication(context.Background(), ApplicationInput{Name: "api", EnvironmentID: "env1"})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if app.ID != "app1" || app.AppName != "api-abc123" {
		t.Errorf("app = %+v", app)
	}
}

func TestWaitForDeployment_PollsUntilDone(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		status := "running"
		if n >= 3 {
			status = "done"
		}
		_ = json.NewEncoder(w).Encode(Application{ID: "app1", ApplicationStatus: status})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	err := c.WaitForDeployment(context.Background(), "app1", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForDeployment() error = %v", err)
	}
	if atomic.LoadInt32(&calls) < 3 {
		t.Errorf("polled %d times, want >= 3", calls)
	}
}

func TestWaitForDeployment_ErrorStatusFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Application{ID: "app1", ApplicationStatus: "error"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	err := c.WaitForDeployment(context.Background(), "app1", 1*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for failed deployment, got nil")
	}
}
```

`internal/client/domain_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateDomain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/domain.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body DomainInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Host != "api.example.com" {
			t.Errorf("host = %q", body.Host)
		}
		_ = json.NewEncoder(w).Encode(Domain{ID: "d1", Host: "api.example.com"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	d, err := c.CreateDomain(context.Background(), DomainInput{
		Host: "api.example.com", ApplicationID: "app1", Port: 8080,
	})
	if err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}
	if d.ID != "d1" {
		t.Errorf("domain.ID = %q, want d1", d.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run "Application|Domain|Deployment" -v`
Expected: FAIL — undefined `Application`, `Domain`, etc.

- [ ] **Step 3: Write `internal/client/application.go`**

```go
package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Application is a Docker-image-sourced application in Dokploy.
type Application struct {
	ID                string `json:"applicationId"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	AppName           string `json:"appName"`
	EnvironmentID     string `json:"environmentId"`
	DockerImage       string `json:"dockerImage"`
	RegistryURL       string `json:"registryUrl"`
	Username          string `json:"username"`
	ApplicationStatus string `json:"applicationStatus"`
	Env               string `json:"env"`
}

// ApplicationInput is the create/update payload (identity fields only).
type ApplicationInput struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	EnvironmentID string `json:"environmentId,omitempty"`
}

// DockerProviderInput configures the Docker image source.
type DockerProviderInput struct {
	ApplicationID string `json:"applicationId"`
	DockerImage   string `json:"dockerImage"`
	RegistryURL   string `json:"registryUrl,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
}

func (c *Client) CreateApplication(ctx context.Context, in ApplicationInput) (*Application, error) {
	var out Application
	if err := c.do(ctx, http.MethodPost, "application.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetApplication(ctx context.Context, id string) (*Application, error) {
	var out Application
	q := url.Values{"applicationId": {id}}
	if err := c.do(ctx, http.MethodGet, "application.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateApplication(ctx context.Context, id string, in ApplicationInput) error {
	payload := struct {
		ApplicationInput
		ID string `json:"applicationId"`
	}{ApplicationInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "application.update", payload, nil, nil)
}

func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	payload := map[string]string{"applicationId": id}
	return c.do(ctx, http.MethodPost, "application.delete", payload, nil, nil)
}

// SaveDockerProvider sets the Docker image source and registry credentials.
func (c *Client) SaveDockerProvider(ctx context.Context, in DockerProviderInput) error {
	return c.do(ctx, http.MethodPost, "application.saveDockerProvider", in, nil, nil)
}

// SaveEnvironment sets the application's environment variables (dotenv string).
func (c *Client) SaveEnvironment(ctx context.Context, applicationID, env string) error {
	payload := map[string]string{"applicationId": applicationID, "env": env}
	return c.do(ctx, http.MethodPost, "application.saveEnvironment", payload, nil, nil)
}

// Deploy triggers an asynchronous deployment.
func (c *Client) Deploy(ctx context.Context, applicationID string) error {
	payload := map[string]string{"applicationId": applicationID}
	return c.do(ctx, http.MethodPost, "application.deploy", payload, nil, nil)
}

// WaitForDeployment polls application status until it reaches "done" or
// "error". It returns an error on "error" status or when ctx is cancelled.
func (c *Client) WaitForDeployment(ctx context.Context, applicationID string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		app, err := c.GetApplication(ctx, applicationID)
		if err != nil {
			return err
		}
		switch app.ApplicationStatus {
		case "done":
			return nil
		case "error":
			return fmt.Errorf("deployment failed (applicationStatus=error); check deploy logs in the Dokploy dashboard for application %q", applicationID)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out or cancelled waiting for deployment of %q: %w", applicationID, ctx.Err())
		case <-ticker.C:
		}
	}
}
```

- [ ] **Step 4: Write `internal/client/domain.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Domain routes external traffic to an application.
type Domain struct {
	ID              string `json:"domainId"`
	Host            string `json:"host"`
	Path            string `json:"path"`
	Port            int    `json:"port"`
	HTTPS           bool   `json:"https"`
	CertificateType string `json:"certificateType"`
	ApplicationID   string `json:"applicationId"`
}

// DomainInput is the writable payload for create/update.
type DomainInput struct {
	Host            string `json:"host"`
	Path            string `json:"path,omitempty"`
	Port            int    `json:"port,omitempty"`
	HTTPS           bool   `json:"https"`
	CertificateType string `json:"certificateType,omitempty"`
	ApplicationID   string `json:"applicationId,omitempty"`
}

func (c *Client) CreateDomain(ctx context.Context, in DomainInput) (*Domain, error) {
	var out Domain
	if err := c.do(ctx, http.MethodPost, "domain.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDomain(ctx context.Context, id string) (*Domain, error) {
	var out Domain
	q := url.Values{"domainId": {id}}
	if err := c.do(ctx, http.MethodGet, "domain.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateDomain(ctx context.Context, id string, in DomainInput) (*Domain, error) {
	payload := struct {
		DomainInput
		ID string `json:"domainId"`
	}{DomainInput: in, ID: id}
	var out Domain
	if err := c.do(ctx, http.MethodPost, "domain.update", payload, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteDomain(ctx context.Context, id string) error {
	payload := map[string]string{"domainId": id}
	return c.do(ctx, http.MethodPost, "domain.delete", payload, nil, nil)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/client/... -v`
Expected: PASS (all client tests).

- [ ] **Step 6: Commit**

```bash
gofmt -w . ; git add internal/client/application*.go internal/client/domain*.go && git commit -m "feat: application and domain API client methods"
```

---

## Task 9: Wire the provider Configure method

**Files:**
- Modify: `internal/provider/provider.go`
- Create: `internal/provider/provider_test.go`

- [ ] **Step 1: Write the failing test**

`internal/provider/provider_test.go`:

```go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used by every acceptance test.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"dokploy": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifies required env vars are set before acceptance tests.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, k := range []string{"DOKPLOY_ENDPOINT", "DOKPLOY_API_KEY"} {
		if v := getEnv(k); v == "" {
			t.Fatalf("%s must be set for acceptance tests", k)
		}
	}
}

func TestProvider_Schema(t *testing.T) {
	p := New("test")()
	if p == nil {
		t.Fatal("New() returned nil provider")
	}
}
```

Add the small `getEnv` helper to `provider.go` in Step 2 (`os.Getenv` wrapper kept for testability).

- [ ] **Step 2: Replace `Configure`, `Resources`, and add helper in `provider.go`**

Replace the placeholder `Configure` and `Resources` methods with:

```go
func (p *dokployProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config dokployProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := config.Endpoint.ValueString()
	if endpoint == "" {
		endpoint = getEnv("DOKPLOY_ENDPOINT")
	}
	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = getEnv("DOKPLOY_API_KEY")
	}

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"),
			"Missing Dokploy endpoint",
			"Set the `endpoint` attribute or the DOKPLOY_ENDPOINT environment variable.")
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"),
			"Missing Dokploy API key",
			"Set the `api_key` attribute or the DOKPLOY_API_KEY environment variable.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c := client.New(endpoint, apiKey)
	resp.ResourceData = c
}

func (p *dokployProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewOrganizationResource,
		NewProjectResource,
		NewEnvironmentResource,
		NewApplicationResource,
		NewDomainResource,
	}
}

// getEnv wraps os.Getenv so tests can reference it through one symbol.
func getEnv(key string) string {
	return os.Getenv(key)
}
```

Add `"os"` and `"github.com/hashicorp/terraform-plugin-framework/path"` and `"github.com/lucasaarch/terraform-provider-dokploy/internal/client"` to the imports.

> The five `New*Resource` constructors are defined in Tasks 10–14. The provider will not compile until at least stub constructors exist. Either implement Tasks 10–14 before running `go build`, or temporarily return `nil` from `Resources` while building incrementally — but the final state of this task requires all five wired.

- [ ] **Step 3: Run the test**

Run: `go test ./internal/provider/ -run TestProvider -v`
Expected: PASS once Tasks 10–14 are complete. If implementing in order, this test compiles after Task 14.

- [ ] **Step 4: Commit**

```bash
gofmt -w . ; git add internal/provider/provider.go internal/provider/provider_test.go && git commit -m "feat: provider configuration with endpoint and api_key"
```

---

## Task 10: Organization resource

**Files:**
- Create: `internal/provider/organization_resource.go`
- Test: `internal/provider/organization_resource_test.go`

- [ ] **Step 1: Write the failing acceptance test**

`internal/provider/organization_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationResource(t *testing.T) {
	name := fmt.Sprintf("tf-acc-org-%d", randInt())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "dokploy_organization" "test" { name = %q }`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_organization.test", "name", name),
					resource.TestCheckResourceAttrSet("dokploy_organization.test", "id"),
				),
			},
			{
				ResourceName:      "dokploy_organization.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: fmt.Sprintf(`resource "dokploy_organization" "test" { name = "%s-renamed" }`, name),
				Check:  resource.TestCheckResourceAttr("dokploy_organization.test", "name", name+"-renamed"),
			},
		},
	})
}
```

Create `internal/provider/helpers_test.go` with the shared `randInt` helper:

```go
package provider

import (
	"math/rand"
	"time"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// randInt returns a positive random int for unique acceptance-test names.
func randInt() int {
	return rng.Intn(1_000_000)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/provider/ -run TestAccOrganizationResource -v` (without `TF_ACC`)
Expected: test is SKIPPED (acceptance tests need `TF_ACC=1`), but the package must still **compile-fail** with `undefined: NewOrganizationResource`. Confirm via `go build ./...`.

- [ ] **Step 3: Write `internal/provider/organization_resource.go`**

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
	_ resource.Resource                = &organizationResource{}
	_ resource.ResourceWithConfigure   = &organizationResource{}
	_ resource.ResourceWithImportState = &organizationResource{}
)

type organizationResource struct {
	client *client.Client
}

// NewOrganizationResource is the resource constructor registered by the provider.
func NewOrganizationResource() resource.Resource {
	return &organizationResource{}
}

type organizationModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (r *organizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *organizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy organization, the top-level tenancy object.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organization name.",
			},
		},
	}
}

func (r *organizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *organizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org, err := r.client.CreateOrganization(ctx, client.OrganizationInput{Name: plan.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization", err.Error())
		return
	}

	plan.ID = types.StringValue(org.ID)
	plan.Name = types.StringValue(org.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *organizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org, err := r.client.GetOrganization(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}

	state.Name = types.StringValue(org.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *organizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan organizationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org, err := r.client.UpdateOrganization(ctx, plan.ID.ValueString(),
		client.OrganizationInput{Name: plan.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization", err.Error())
		return
	}

	plan.Name = types.StringValue(org.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *organizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteOrganization(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting organization",
			"Dokploy may block deletion of the active or last organization. Underlying error: "+err.Error())
		return
	}
}

func (r *organizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Build and run acceptance test**

Run: `go build ./...` — expected: compiles.
Run: `TF_ACC=1 DOKPLOY_ENDPOINT=... DOKPLOY_API_KEY=... go test ./internal/provider/ -run TestAccOrganizationResource -v`
Expected: PASS (3 test steps).

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/provider/organization_resource.go internal/provider/organization_resource_test.go internal/provider/helpers_test.go && git commit -m "feat: dokploy_organization resource"
```

---

## Task 11: Project resource

**Files:**
- Create: `internal/provider/project_resource.go`
- Test: `internal/provider/project_resource_test.go`

- [ ] **Step 1: Write the failing acceptance test**

`internal/provider/project_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProjectResource(t *testing.T) {
	name := fmt.Sprintf("tf-acc-proj-%d", randInt())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokploy_project" "test" {
  name        = %q
  description = "created by acceptance test"
  production_env = {
    LOG_LEVEL = "info"
  }
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_project.test", "name", name),
					resource.TestCheckResourceAttrSet("dokploy_project.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_project.test", "production_environment_id"),
					resource.TestCheckResourceAttr("dokploy_project.test", "production_env.LOG_LEVEL", "info"),
				),
			},
			{
				ResourceName:            "dokploy_project.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"production_env"},
			},
		},
	})
}
```

> `production_env` is in `ImportStateVerifyIgnore` because environment-level shared variables are written through a separate endpoint and may not round-trip identically on import; the `id` and `production_environment_id` are the import-critical attributes.

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./...`
Expected: FAIL — `undefined: NewProjectResource`.

- [ ] **Step 3: Write `internal/provider/project_resource.go`**

The production environment's shared variables are written via `UpdateEnvironment` on the project's production environment id.

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
	_ resource.Resource                = &projectResource{}
	_ resource.ResourceWithConfigure   = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
)

type projectResource struct {
	client *client.Client
}

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

type projectModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	OrganizationID          types.String `tfsdk:"organization_id"`
	ProductionEnv           types.Map    `tfsdk:"production_env"`
	ProductionEnvironmentID types.String `tfsdk:"production_environment_id"`
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy project. Each project owns an auto-created `production` environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project description.",
			},
			"organization_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization the project belongs to. Defaults to the API key's organization.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"production_env": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Shared environment variables for the auto-created `production` environment.",
			},
			"production_environment_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier of the auto-created `production` environment.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

// envMapToString converts a types.Map into a dotenv string.
func envMapToString(ctx context.Context, m types.Map) (string, error) {
	if m.IsNull() || m.IsUnknown() {
		return "", nil
	}
	raw := map[string]string{}
	if diags := m.ElementsAs(ctx, &raw, false); diags.HasError() {
		return "", fmt.Errorf("converting env map")
	}
	return client.MapToDotenv(raw), nil
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	proj, err := r.client.CreateProject(ctx, client.ProjectInput{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		OrganizationID: plan.OrganizationID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	prodEnvID := proj.ProductionEnvironmentID()

	// Apply production_env to the auto-created production environment.
	if !plan.ProductionEnv.IsNull() {
		envStr, convErr := envMapToString(ctx, plan.ProductionEnv)
		if convErr != nil {
			resp.Diagnostics.AddError("Error reading production_env", convErr.Error())
			return
		}
		if _, err := r.client.UpdateEnvironment(ctx, prodEnvID, client.EnvironmentInput{Env: envStr}); err != nil {
			resp.Diagnostics.AddError("Error setting production_env", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(proj.ID)
	plan.OrganizationID = types.StringValue(proj.OrganizationID)
	plan.ProductionEnvironmentID = types.StringValue(prodEnvID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	proj, err := r.client.GetProject(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	state.Name = types.StringValue(proj.Name)
	if proj.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(proj.Description)
	}
	state.OrganizationID = types.StringValue(proj.OrganizationID)
	state.ProductionEnvironmentID = types.StringValue(proj.ProductionEnvironmentID())
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.UpdateProject(ctx, plan.ID.ValueString(), client.ProjectInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}

	envStr, convErr := envMapToString(ctx, plan.ProductionEnv)
	if convErr != nil {
		resp.Diagnostics.AddError("Error reading production_env", convErr.Error())
		return
	}
	if _, err := r.client.UpdateEnvironment(ctx, plan.ProductionEnvironmentID.ValueString(),
		client.EnvironmentInput{Env: envStr}); err != nil {
		resp.Diagnostics.AddError("Error updating production_env", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteProject(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Build and run acceptance test**

Run: `go build ./...` then
`TF_ACC=1 DOKPLOY_ENDPOINT=... DOKPLOY_API_KEY=... go test ./internal/provider/ -run TestAccProjectResource -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/provider/project_resource.go internal/provider/project_resource_test.go && git commit -m "feat: dokploy_project resource"
```

---

## Task 12: Environment resource

**Files:**
- Create: `internal/provider/environment_resource.go`
- Test: `internal/provider/environment_resource_test.go`

- [ ] **Step 1: Write the failing acceptance test**

`internal/provider/environment_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnvironmentResource(t *testing.T) {
	suffix := randInt()
	config := func(level string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-env-proj-%d"
}

resource "dokploy_environment" "test" {
  project_id  = dokploy_project.test.id
  name        = "staging"
  description = "acc test environment"
  env = {
    LOG_LEVEL = %q
  }
}`, suffix, level)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("debug"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_environment.test", "name", "staging"),
					resource.TestCheckResourceAttrSet("dokploy_environment.test", "id"),
					resource.TestCheckResourceAttr("dokploy_environment.test", "env.LOG_LEVEL", "debug"),
				),
			},
			{
				ResourceName:      "dokploy_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config("warn"),
				Check:  resource.TestCheckResourceAttr("dokploy_environment.test", "env.LOG_LEVEL", "warn"),
			},
		},
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./...`
Expected: FAIL — `undefined: NewEnvironmentResource`.

- [ ] **Step 3: Write `internal/provider/environment_resource.go`**

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
	_ resource.Resource                = &environmentResource{}
	_ resource.ResourceWithConfigure   = &environmentResource{}
	_ resource.ResourceWithImportState = &environmentResource{}
)

type environmentResource struct {
	client *client.Client
}

func NewEnvironmentResource() resource.Resource {
	return &environmentResource{}
}

type environmentModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Env         types.Map    `tfsdk:"env"`
}

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A custom (non-production) environment inside a Dokploy project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project the environment belongs to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Environment name, e.g. `staging`. Do not use `production` (managed via dokploy_project).",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Environment description.",
			},
			"env": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Shared environment variables for all applications in this environment.",
			},
		},
	}
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envStr, convErr := envMapToString(ctx, plan.Env)
	if convErr != nil {
		resp.Diagnostics.AddError("Error reading env", convErr.Error())
		return
	}

	env, err := r.client.CreateEnvironment(ctx, client.EnvironmentInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		ProjectID:   plan.ProjectID.ValueString(),
		Env:         envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating environment", err.Error())
		return
	}

	plan.ID = types.StringValue(env.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.GetEnvironment(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading environment", err.Error())
		return
	}

	state.Name = types.StringValue(env.Name)
	state.ProjectID = types.StringValue(env.ProjectID)
	if env.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(env.Description)
	}
	if !state.Env.IsNull() {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(env.Env))
		resp.Diagnostics.Append(diags...)
		state.Env = envMap
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envStr, convErr := envMapToString(ctx, plan.Env)
	if convErr != nil {
		resp.Diagnostics.AddError("Error reading env", convErr.Error())
		return
	}

	if _, err := r.client.UpdateEnvironment(ctx, plan.ID.ValueString(), client.EnvironmentInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Env:         envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating environment", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteEnvironment(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting environment", err.Error())
	}
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Build and run acceptance test**

Run: `go build ./...` then
`TF_ACC=1 DOKPLOY_ENDPOINT=... DOKPLOY_API_KEY=... go test ./internal/provider/ -run TestAccEnvironmentResource -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/provider/environment_resource.go internal/provider/environment_resource_test.go && git commit -m "feat: dokploy_environment resource"
```

---

## Task 13: Application resource (with deploy orchestration)

**Files:**
- Create: `internal/provider/application_resource.go`
- Test: `internal/provider/application_resource_test.go`

- [ ] **Step 1: Write the failing acceptance test**

`internal/provider/application_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccApplicationResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-app-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-app"
  docker_image   = %q
  env = {
    APP_ENV = "test"
  }
  timeouts {
    create = "15m"
    update = "15m"
  }
}`, suffix, image)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("nginx:1.27"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_application.test", "docker_image", "nginx:1.27"),
					resource.TestCheckResourceAttrSet("dokploy_application.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_application.test", "app_name"),
					resource.TestCheckResourceAttr("dokploy_application.test", "status", "done"),
				),
			},
			{
				ResourceName:            "dokploy_application.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"registry_password", "timeouts"},
			},
			{
				Config: config("nginx:1.28"),
				Check:  resource.TestCheckResourceAttr("dokploy_application.test", "docker_image", "nginx:1.28"),
			},
		},
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./...`
Expected: FAIL — `undefined: NewApplicationResource`.

- [ ] **Step 3: Write `internal/provider/application_resource.go`**

The resource orchestrates create → saveDockerProvider → saveEnvironment → deploy → poll. `registry_password` is never overwritten by Read (the API does not return it; see spec).

```go
package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

const (
	defaultDeployTimeout = 10 * time.Minute
	deployPollInterval   = 5 * time.Second
)

var (
	_ resource.Resource                = &applicationResource{}
	_ resource.ResourceWithConfigure   = &applicationResource{}
	_ resource.ResourceWithImportState = &applicationResource{}
)

type applicationResource struct {
	client *client.Client
}

func NewApplicationResource() resource.Resource {
	return &applicationResource{}
}

type applicationModel struct {
	ID               types.String   `tfsdk:"id"`
	EnvironmentID    types.String   `tfsdk:"environment_id"`
	Name             types.String   `tfsdk:"name"`
	Description      types.String   `tfsdk:"description"`
	DockerImage      types.String   `tfsdk:"docker_image"`
	RegistryURL      types.String   `tfsdk:"registry_url"`
	RegistryUsername types.String   `tfsdk:"registry_username"`
	RegistryPassword types.String   `tfsdk:"registry_password"`
	Env              types.Map      `tfsdk:"env"`
	AppName          types.String   `tfsdk:"app_name"`
	Status           types.String   `tfsdk:"status"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *applicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (r *applicationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy application sourced from a Docker image. Applying this resource deploys the application and waits for it to finish.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Environment the application belongs to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Application description.",
			},
			"docker_image": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Docker image to deploy, e.g. `nginx:1.27`.",
			},
			"registry_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Private registry URL. Omit for public images.",
			},
			"registry_username": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Registry username.",
			},
			"registry_password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Registry password. Not returned by the API; drift on this field is not detected.",
			},
			"env": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Application environment variables.",
			},
			"app_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal application name generated by Dokploy.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of the most recent deployment.",
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *applicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

// configureAndDeploy applies docker provider config + env, triggers a deploy,
// and waits for it to finish. Used by both Create and Update.
func (r *applicationResource) configureAndDeploy(ctx context.Context, m *applicationModel, timeout time.Duration) error {
	id := m.ID.ValueString()

	if err := r.client.SaveDockerProvider(ctx, client.DockerProviderInput{
		ApplicationID: id,
		DockerImage:   m.DockerImage.ValueString(),
		RegistryURL:   m.RegistryURL.ValueString(),
		Username:      m.RegistryUsername.ValueString(),
		Password:      m.RegistryPassword.ValueString(),
	}); err != nil {
		return fmt.Errorf("saving docker provider: %w", err)
	}

	envStr, err := envMapToString(ctx, m.Env)
	if err != nil {
		return err
	}
	if err := r.client.SaveEnvironment(ctx, id, envStr); err != nil {
		return fmt.Errorf("saving environment: %w", err)
	}

	if err := r.client.Deploy(ctx, id); err != nil {
		return fmt.Errorf("triggering deploy: %w", err)
	}

	deployCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return r.client.WaitForDeployment(deployCtx, id, deployPollInterval)
}

func (r *applicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan applicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultDeployTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.CreateApplication(ctx, client.ApplicationInput{
		Name:          plan.Name.ValueString(),
		Description:   plan.Description.ValueString(),
		EnvironmentID: plan.EnvironmentID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating application", err.Error())
		return
	}
	plan.ID = types.StringValue(app.ID)
	plan.AppName = types.StringValue(app.AppName)

	if err := r.configureAndDeploy(ctx, &plan, createTimeout); err != nil {
		// The application exists; persist its id so a later apply can retry.
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Application deploy failed", err.Error())
		return
	}

	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *applicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state applicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.GetApplication(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading application", err.Error())
		return
	}

	state.Name = types.StringValue(app.Name)
	state.EnvironmentID = types.StringValue(app.EnvironmentID)
	state.DockerImage = types.StringValue(app.DockerImage)
	state.AppName = types.StringValue(app.AppName)
	state.Status = types.StringValue(app.ApplicationStatus)
	if app.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(app.Description)
	}
	if app.RegistryURL != "" || !state.RegistryURL.IsNull() {
		state.RegistryURL = types.StringValue(app.RegistryURL)
	}
	if app.Username != "" || !state.RegistryUsername.IsNull() {
		state.RegistryUsername = types.StringValue(app.Username)
	}
	// registry_password is intentionally NOT updated: the API does not return it.
	if !state.Env.IsNull() {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(app.Env))
		resp.Diagnostics.Append(diags...)
		state.Env = envMap
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *applicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan applicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDeployTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateApplication(ctx, plan.ID.ValueString(), client.ApplicationInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Error updating application", err.Error())
		return
	}

	if err := r.configureAndDeploy(ctx, &plan, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Application deploy failed", err.Error())
		return
	}

	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *applicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state applicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteApplication(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting application", err.Error())
	}
}

func (r *applicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Build and run acceptance test**

Run: `go build ./...` then
`TF_ACC=1 DOKPLOY_ENDPOINT=... DOKPLOY_API_KEY=... go test ./internal/provider/ -run TestAccApplicationResource -v -timeout 30m`
Expected: PASS (real deploy of `nginx`).

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/provider/application_resource.go internal/provider/application_resource_test.go && git commit -m "feat: dokploy_application resource with deploy orchestration"
```

---

## Task 14: Domain resource

**Files:**
- Create: `internal/provider/domain_resource.go`
- Test: `internal/provider/domain_resource_test.go`

- [ ] **Step 1: Write the failing acceptance test**

`internal/provider/domain_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDomainResource(t *testing.T) {
	suffix := randInt()
	host := fmt.Sprintf("tf-acc-%d.example.com", suffix)
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-domain-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-domain-app"
  docker_image   = "nginx:1.27"
}

resource "dokploy_domain" "test" {
  application_id   = dokploy_application.test.id
  host             = %q
  port             = 80
  https            = false
  certificate_type = "none"
}`, suffix, host)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_domain.test", "host", host),
					resource.TestCheckResourceAttr("dokploy_domain.test", "port", "80"),
					resource.TestCheckResourceAttrSet("dokploy_domain.test", "id"),
				),
			},
			{
				ResourceName:      "dokploy_domain.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./...`
Expected: FAIL — `undefined: NewDomainResource`.

- [ ] **Step 3: Write `internal/provider/domain_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &domainResource{}
	_ resource.ResourceWithConfigure   = &domainResource{}
	_ resource.ResourceWithImportState = &domainResource{}
)

type domainResource struct {
	client *client.Client
}

func NewDomainResource() resource.Resource {
	return &domainResource{}
}

type domainModel struct {
	ID              types.String `tfsdk:"id"`
	ApplicationID   types.String `tfsdk:"application_id"`
	Host            types.String `tfsdk:"host"`
	Path            types.String `tfsdk:"path"`
	Port            types.Int64  `tfsdk:"port"`
	HTTPS           types.Bool   `tfsdk:"https"`
	CertificateType types.String `tfsdk:"certificate_type"`
}

func (r *domainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *domainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A domain routing external traffic to a Dokploy application.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"application_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application the domain routes to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Domain hostname, e.g. `api.example.com`.",
			},
			"path": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("/"),
				MarkdownDescription: "Path prefix to route. Defaults to `/`.",
			},
			"port": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Container port to route to.",
			},
			"https": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether to serve over HTTPS.",
			},
			"certificate_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("none"),
				MarkdownDescription: "Certificate type: `none` or `letsencrypt`.",
			},
		},
	}
}

func (r *domainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (m domainModel) toInput() client.DomainInput {
	return client.DomainInput{
		Host:            m.Host.ValueString(),
		Path:            m.Path.ValueString(),
		Port:            int(m.Port.ValueInt64()),
		HTTPS:           m.HTTPS.ValueBool(),
		CertificateType: m.CertificateType.ValueString(),
		ApplicationID:   m.ApplicationID.ValueString(),
	}
}

func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.CreateDomain(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating domain", err.Error())
		return
	}

	plan.ID = types.StringValue(domain.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.GetDomain(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading domain", err.Error())
		return
	}

	state.ApplicationID = types.StringValue(domain.ApplicationID)
	state.Host = types.StringValue(domain.Host)
	state.Path = types.StringValue(domain.Path)
	state.Port = types.Int64Value(int64(domain.Port))
	state.HTTPS = types.BoolValue(domain.HTTPS)
	state.CertificateType = types.StringValue(domain.CertificateType)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.UpdateDomain(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating domain", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDomain(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting domain", err.Error())
	}
}

func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Build, run full unit suite and acceptance test**

Run: `go build ./...`
Run: `go test ./internal/client/... -v` — expected: PASS.
Run: `TF_ACC=1 DOKPLOY_ENDPOINT=... DOKPLOY_API_KEY=... go test ./internal/provider/ -run TestAccDomainResource -v -timeout 30m`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add internal/provider/domain_resource.go internal/provider/domain_resource_test.go && git commit -m "feat: dokploy_domain resource"
```

---

## Task 15: Examples and generated documentation

**Files:**
- Create: `examples/provider/provider.tf`
- Create: `examples/resources/dokploy_organization/resource.tf`
- Create: `examples/resources/dokploy_project/resource.tf`
- Create: `examples/resources/dokploy_environment/resource.tf`
- Create: `examples/resources/dokploy_application/resource.tf`
- Create: `examples/resources/dokploy_domain/resource.tf`
- Create: `examples/resources/dokploy_organization/import.sh` (and one per resource)
- Modify: `internal/provider/provider.go` (add `go:generate` directive)

- [ ] **Step 1: Create `examples/provider/provider.tf`**

```hcl
terraform {
  required_providers {
    dokploy = {
      source = "lucasaarch/dokploy"
    }
  }
}

provider "dokploy" {
  endpoint = "https://dokploy.example.com"
  # api_key is read from the DOKPLOY_API_KEY environment variable.
}
```

- [ ] **Step 2: Create each resource example**

`examples/resources/dokploy_organization/resource.tf`:

```hcl
resource "dokploy_organization" "main" {
  name = "my-company"
}
```

`examples/resources/dokploy_project/resource.tf`:

```hcl
resource "dokploy_project" "app" {
  name            = "my-app"
  description     = "Main application project"
  organization_id = dokploy_organization.main.id

  production_env = {
    LOG_LEVEL = "info"
  }
}
```

`examples/resources/dokploy_environment/resource.tf`:

```hcl
resource "dokploy_environment" "staging" {
  project_id  = dokploy_project.app.id
  name        = "staging"
  description = "Staging environment"

  env = {
    LOG_LEVEL = "debug"
  }
}
```

`examples/resources/dokploy_application/resource.tf`:

```hcl
resource "dokploy_application" "api" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "api"
  docker_image   = "nginx:1.27"

  env = {
    PORT = "8080"
  }

  timeouts {
    create = "15m"
    update = "15m"
  }
}
```

`examples/resources/dokploy_domain/resource.tf`:

```hcl
resource "dokploy_domain" "web" {
  application_id   = dokploy_application.api.id
  host             = "api.example.com"
  port             = 8080
  https            = true
  certificate_type = "letsencrypt"
}
```

- [ ] **Step 3: Create import scripts**

Create `examples/resources/dokploy_organization/import.sh`:

```bash
terraform import dokploy_organization.main <organizationId>
```

Create the equivalent `import.sh` for the other four resources, substituting the
resource type, local name, and id placeholder (`<projectId>`, `<environmentId>`,
`<applicationId>`, `<domainId>`).

- [ ] **Step 4: Add the `go:generate` directive**

Add this line to `internal/provider/provider.go`, directly above the `package provider` line:

```go
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name dokploy
```

- [ ] **Step 5: Generate docs**

Run: `go generate ./...`
Expected: a `docs/` directory is created with `index.md` and one page per resource.

- [ ] **Step 6: Commit**

```bash
gofmt -w . ; git add examples docs internal/provider/provider.go && git commit -m "docs: examples and generated provider documentation"
```

---

## Task 16: GoReleaser and GitHub Actions CI

**Files:**
- Create: `.goreleaser.yml`
- Create: `terraform-registry-manifest.json`
- Create: `.github/workflows/test.yml`
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create `terraform-registry-manifest.json`**

```json
{
  "version": 1,
  "metadata": {
    "protocol_versions": ["6.0"]
  }
}
```

- [ ] **Step 2: Create `.goreleaser.yml`**

This is the standard HashiCorp provider release configuration.

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - '-s -w -X main.version={{.Version}}'
    goos:
      - linux
      - darwin
      - windows
      - freebsd
    goarch:
      - amd64
      - arm64
      - arm
    ignore:
      - goos: windows
        goarch: arm
    binary: '{{ .ProjectName }}_v{{ .Version }}'

archives:
  - formats: [zip]
    name_template: '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}'

checksum:
  name_template: '{{ .ProjectName }}_{{ .Version }}_SHA256SUMS'
  algorithm: sha256

signs:
  - artifacts: checksum
    args:
      - "--batch"
      - "--local-user"
      - "{{ .Env.GPG_FINGERPRINT }}"
      - "--output"
      - "${signature}"
      - "--detach-sign"
      - "${artifact}"

release:
  extra_files:
    - glob: 'terraform-registry-manifest.json'
      name_template: '{{ .ProjectName }}_{{ .Version }}_manifest.json'

changelog:
  disable: true
```

- [ ] **Step 3: Create `.github/workflows/test.yml`**

```yaml
name: Test

on:
  pull_request:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - run: go mod download
      - run: go build ./...
      - run: go vet ./...

  unit-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - run: go test ./internal/client/... -v

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - uses: golangci/golangci-lint-action@v6

  docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - run: go generate ./...
      - name: Check docs are up to date
        run: |
          git diff --exit-code -- docs/ \
            || (echo "docs/ is out of date; run 'go generate ./...'" && exit 1)
```

- [ ] **Step 4: Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Import GPG key
        uses: crazy-max/ghaction-import-gpg@v6
        id: import_gpg
        with:
          gpg_private_key: ${{ secrets.GPG_PRIVATE_KEY }}
          passphrase: ${{ secrets.PASSPHRASE }}
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          args: release --clean
        env:
          GPG_FINGERPRINT: ${{ steps.import_gpg.outputs.fingerprint }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 5: Verify GoReleaser config**

Run: `go run github.com/goreleaser/goreleaser/v2@latest check`
Expected: `.goreleaser.yml` is valid. (Skip if GoReleaser cannot be fetched in the environment; CI will validate it.)

- [ ] **Step 6: Commit**

```bash
gofmt -w . ; git add .goreleaser.yml terraform-registry-manifest.json .github && git commit -m "ci: GoReleaser release pipeline and GitHub Actions"
```

---

## Task 17: README, LICENSE, and end-to-end acceptance test

**Files:**
- Create: `LICENSE`
- Create: `README.md`
- Test: `internal/provider/e2e_test.go`

- [ ] **Step 1: Create `LICENSE`**

Write the full text of the Mozilla Public License 2.0 into `LICENSE`. Fetch it from `https://www.mozilla.org/media/MPL/2.0/index.txt`.

- [ ] **Step 2: Create `README.md`**

```markdown
# Terraform Provider for Dokploy

Manage [Dokploy](https://dokploy.com) infrastructure declaratively with Terraform.

## Resources

- `dokploy_organization` — top-level tenancy
- `dokploy_project` — project plus its auto-created `production` environment
- `dokploy_environment` — custom environments
- `dokploy_application` — Docker-image applications (deploys on apply)
- `dokploy_domain` — domains routing traffic to applications

## Provider configuration

```hcl
provider "dokploy" {
  endpoint = "https://dokploy.example.com" # or DOKPLOY_ENDPOINT
  # api_key via DOKPLOY_API_KEY
}
```

## Development

- `make build` — build the provider binary
- `make test` — run unit tests (no network)
- `make testacc` — run acceptance tests (needs `DOKPLOY_ENDPOINT` and `DOKPLOY_API_KEY`)
- `make docs` — regenerate documentation

## License

MPL-2.0
```

- [ ] **Step 3: Write the end-to-end acceptance test**

`internal/provider/e2e_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccEndToEnd builds the full resource graph in one apply:
// organization -> project -> environment + application -> domain.
func TestAccEndToEnd(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_organization" "e2e" {
  name = "tf-acc-e2e-org-%d"
}

resource "dokploy_project" "e2e" {
  name            = "tf-acc-e2e-proj-%d"
  organization_id = dokploy_organization.e2e.id
  production_env  = { LOG_LEVEL = "info" }
}

resource "dokploy_environment" "e2e" {
  project_id = dokploy_project.e2e.id
  name       = "staging"
  env        = { LOG_LEVEL = "debug" }
}

resource "dokploy_application" "e2e" {
  environment_id = dokploy_project.e2e.production_environment_id
  name           = "tf-acc-e2e-app"
  docker_image   = "nginx:1.27"
  timeouts { create = "15m" }
}

resource "dokploy_domain" "e2e" {
  application_id = dokploy_application.e2e.id
  host           = "tf-acc-e2e-%d.example.com"
  port           = 80
}`, suffix, suffix, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_organization.e2e", "id"),
					resource.TestCheckResourceAttrSet("dokploy_project.e2e", "id"),
					resource.TestCheckResourceAttrSet("dokploy_environment.e2e", "id"),
					resource.TestCheckResourceAttr("dokploy_application.e2e", "status", "done"),
					resource.TestCheckResourceAttrSet("dokploy_domain.e2e", "id"),
				),
			},
		},
	})
}
```

- [ ] **Step 4: Run the full suite**

Run: `go test ./internal/client/... -v` — expected: PASS.
Run: `TF_ACC=1 DOKPLOY_ENDPOINT=... DOKPLOY_API_KEY=... go test ./internal/provider/ -run TestAccEndToEnd -v -timeout 30m`
Expected: PASS — the full graph is created and destroyed.

- [ ] **Step 5: Commit**

```bash
gofmt -w . ; git add LICENSE README.md internal/provider/e2e_test.go && git commit -m "docs: README, MPL-2.0 license, end-to-end acceptance test"
```

---

## Final verification checklist

After all tasks, confirm:

- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/client/... -v` — all unit tests pass
- [ ] `go vet ./...` clean
- [ ] `golangci-lint run` clean
- [ ] `go generate ./...` produces no uncommitted diff
- [ ] Full acceptance suite passes against the live instance: `make testacc`
- [ ] `.goreleaser.yml` validates

## Post-implementation: Terraform Registry publishing (manual, one-time)

Not code — operational steps for the maintainer:

1. Push the repo to GitHub as `lucasaarch/terraform-provider-dokploy` (public).
2. Generate a GPG key; add the **public** key to the Terraform Registry account.
3. Add repo secrets `GPG_PRIVATE_KEY` and `PASSPHRASE`.
4. Sign in to registry.terraform.io, publish the provider from the GitHub repo.
5. Tag a release: `git tag v0.1.0 && git push origin v0.1.0` — the release workflow builds, signs, and publishes; the Registry picks it up automatically.
