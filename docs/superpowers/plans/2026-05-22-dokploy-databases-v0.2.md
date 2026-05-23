# Dokploy Database Resources (v0.2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add five managed database resources (`dokploy_postgres`, `dokploy_mysql`, `dokploy_mariadb`, `dokploy_mongo`, `dokploy_redis`) to the provider, each with the same create + deploy + poll lifecycle that `dokploy_application` already uses, and ship as v0.2.0.

**Architecture:** Each database is a thin Terraform resource that wraps a typed client in `internal/client/<db>.go`. Lifecycle orchestration (deploy + status polling) and password generation live in a new shared `internal/provider/database_helpers.go` so the five resources stay small and uniform.

**Tech Stack:** Go 1.26, `terraform-plugin-framework`, `terraform-plugin-framework-timeouts`, `terraform-plugin-testing`, `crypto/rand` for password generation. No new external dependencies.

**Spec:** `docs/superpowers/specs/2026-05-22-dokploy-databases-v0.2-design.md`

---

## Conventions for every task

- TDD: write the failing test first, see it fail, implement, see it pass, commit.
- Run `gofmt -w .` before every commit.
- Commit messages: conventional commits (`feat:`, `test:`, `chore:`, `docs:`).
- Unit tests use `httptest` and need no network. Acceptance tests (`TestAcc*`) run only when `TF_ACC=1` with `DOKPLOY_ENDPOINT` and `DOKPLOY_API_KEY` set (`source .dokploy-test-env` to load).
- End every commit message body with: `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>`.
- Acceptance tests hit the user's real Dokploy instance and create/destroy real resources. All test resource names use the `tf-acc-` prefix. After running, verify the instance is clean of `tf-acc-*` items.
- API.md (`internal/client/API.md`) is the source of truth for endpoint shapes. Where this plan's code differs from API.md, API.md wins — adjust accordingly.

---

## Task 1: Verify the five database routers against the live API

Exploratory probe to confirm endpoint names and payloads for `postgres.*`, `mysql.*`, `mariadb.*`, `mongo.*`, `redis.*`. The plan code in later tasks assumes a layout analogous to `application.*` (already verified in v0.1's `API.md`); this task validates or corrects that assumption before any DB code is written.

**Files:**
- Modify: `internal/client/API.md`

- [ ] **Step 1: Load credentials and confirm reads**

```bash
cd /Users/lukearch/Projects/My/dokploy-terraform-provider
source .dokploy-test-env
for r in postgres mysql mariadb mongo redis; do
  echo "== $r.all =="
  /usr/bin/curl -s -m 20 -o /tmp/$r.json -w "HTTP %{http_code}\n" \
    -H "x-api-key: $DOKPLOY_API_KEY" "$DOKPLOY_ENDPOINT/api/$r.all"
  head -c 400 /tmp/$r.json; echo
done
```

If any returns 404, try the obvious alternative (`<db>.list`, `<db>.findAll`) and note which name works.

- [ ] **Step 2: Probe `create`, `one`, `update`, `deploy`, `remove` for each router**

For each of the five DB types, create a throwaway instance, fetch it via `<db>.one`, update one trivial attribute, deploy it once, then delete it. Record exact HTTP method, path, request body fields, and response body shape for each endpoint.

Example template (substitute `postgres` etc.):

```bash
source .dokploy-test-env
# Adapt the JSON body once you see the first 400 response from saveDockerProvider
# for the application — the validation errors tell you which fields are required.
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d '{"name":"tf-probe-pg","appName":"tf-probe-pg","environmentId":"<env-id>","dockerImage":"postgres:16","databaseName":"probe","databaseUser":"probe","databasePassword":"probepass1234"}' \
  "$DOKPLOY_ENDPOINT/api/postgres.create" | head -c 800
```

Look at the Zod error responses (status 400) to discover required-but-nullable fields — exactly how `saveDockerProvider`/`saveEnvironment` were discovered in v0.1.

- [ ] **Step 3: Confirm the deployment-status field for databases**

After `<db>.deploy`, poll `<db>.one?<id>=...` and record the values that `applicationStatus` takes (`idle`/`running`/`done`/`error` are expected from the application pattern; databases may add `stopped`). Record which values are terminal.

- [ ] **Step 4: Confirm password fields on read**

Call `<db>.one` and check whether `databasePassword` (and `databaseRootPassword` for MySQL/MariaDB) come back in the response, or are stripped. This decides whether the resource Read overwrites state from the API or preserves state.

- [ ] **Step 5: Clean up every probe resource**

Each `<db>.remove` payload is the id of the resource. Verify in `project.all` afterwards that no `tf-probe-*` items remain.

- [ ] **Step 6: Update `internal/client/API.md`**

Append five new sections — `## postgres.*`, `## mysql.*`, `## mariadb.*`, `## mongo.*`, `## redis.*` — each documenting:
- HTTP method + full path of `create`, `one`, `update`, `deploy`, `remove`
- Request body JSON shape (with comments noting required-but-nullable fields)
- Response body JSON shape for `one`
- Whether `databasePassword` / `databaseRootPassword` are returned by `one`
- Observed `applicationStatus` values

Also append a row to the "Deployment Status Reference" table for each new terminal state, if any.

- [ ] **Step 7: Commit**

```bash
git add internal/client/API.md
git commit -m "docs: API reference for postgres/mysql/mariadb/mongo/redis routers"
```

---

## Task 2: Shared database helpers

Move existing `slugify` out of `application_resource.go` into a new shared file, and add `generatePassword` and `deployAndWait` for reuse across the five DB resources.

**Files:**
- Create: `internal/provider/database_helpers.go`
- Create: `internal/provider/database_helpers_test.go`
- Modify: `internal/provider/application_resource.go` (remove the local `slugify`, import it from the shared file)

- [ ] **Step 1: Write failing tests**

`internal/provider/database_helpers_test.go`:

```go
package provider

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"":           "app",
		"  ":         "app",
		"Hello":      "hello",
		"Hello-World": "hello-world",
		"app name":   "app-name",
		"weird!@#":   "weird",
		"-leading-":  "leading",
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider/ -run "TestSlugify|TestGeneratePassword|TestDeployAndWait" -v`
Expected: FAIL — `undefined: generatePassword`, `undefined: deployAndWait` (slugify already exists in `application_resource.go`, so that test may compile-fail because we will move it).

- [ ] **Step 3: Write `internal/provider/database_helpers.go`**

```go
package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
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
```

- [ ] **Step 4: Remove `slugify` from `application_resource.go`**

`slugify` currently lives in `internal/provider/application_resource.go` (added during v0.1). Find and delete the function declaration there (it will be the same code as in Step 3, now in `database_helpers.go`). The `application_resource.go` file already imports `strings`; if removing `slugify` leaves no `strings` reference, also remove the import. Run `gofmt -w .` afterwards.

To find the exact lines:

```bash
grep -n 'func slugify' internal/provider/application_resource.go
```

Delete from `// slugify turns ...` (or the function signature line if no comment) through the closing `}` of the function.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/provider/ -run "TestSlugify|TestGeneratePassword|TestDeployAndWait" -v`
Expected: PASS (5 sub-tests).

Run: `go build ./...`
Expected: clean build (application_resource.go now references the package-level `slugify`).

- [ ] **Step 6: Commit**

```bash
gofmt -w .
git add internal/provider/database_helpers.go internal/provider/database_helpers_test.go internal/provider/application_resource.go
git commit -m "feat: shared database helpers (slugify, generatePassword, deployAndWait)"
```

---

## Task 3: Postgres resource

**Files:**
- Create: `internal/client/postgres.go`
- Create: `internal/client/postgres_test.go`
- Create: `internal/provider/postgres_resource.go`
- Create: `internal/provider/postgres_resource_test.go`

Adjust JSON field names and endpoint paths to match `internal/client/API.md` (updated in Task 1) where this plan's assumptions differ.

- [ ] **Step 1: Write failing client unit tests**

`internal/client/postgres_test.go`:

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

func TestCreatePostgres(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/postgres.create" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		var body PostgresInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DatabaseUser != "app" {
			t.Errorf("databaseUser = %q", body.DatabaseUser)
		}
		_ = json.NewEncoder(w).Encode(Postgres{
			ID: "pg1", Name: "db", AppName: "db-abc",
			DatabaseName: "app", DatabaseUser: "app", DatabasePassword: "secret",
			ApplicationStatus: "idle",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	pg, err := c.CreatePostgres(context.Background(), PostgresInput{
		Name: "db", AppName: "db", EnvironmentID: "env",
		DockerImage:      "postgres:16",
		DatabaseName:     "app",
		DatabaseUser:     "app",
		DatabasePassword: "secret",
	})
	if err != nil {
		t.Fatalf("CreatePostgres() error = %v", err)
	}
	if pg.ID != "pg1" || pg.AppName != "db-abc" {
		t.Errorf("pg = %+v", pg)
	}
}

func TestGetPostgres_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	_, err := c.GetPostgres(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false, want true (err = %v)", err)
	}
}

func TestWaitForPostgresDeployment_Done(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		status := "running"
		if n >= 2 {
			status = "done"
		}
		_ = json.NewEncoder(w).Encode(Postgres{ID: "pg1", ApplicationStatus: status})
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	// statusFn closure pattern mirrors how the provider helper calls it.
	statusFn := func(ctx context.Context) (string, error) {
		pg, err := c.GetPostgres(ctx, "pg1")
		if err != nil {
			return "", err
		}
		return pg.ApplicationStatus, nil
	}
	got, err := statusFn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "running" {
		t.Errorf("first status = %q, want running", got)
	}
	got, _ = statusFn(context.Background())
	got, _ = statusFn(context.Background())
	if got != "done" {
		t.Errorf("third status = %q, want done", got)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Postgres -v`
Expected: FAIL — `undefined: Postgres`, `undefined: PostgresInput`.

- [ ] **Step 3: Write `internal/client/postgres.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Postgres is a Dokploy-managed PostgreSQL service.
type Postgres struct {
	ID                string `json:"postgresId"`
	Name              string `json:"name"`
	AppName           string `json:"appName"`
	Description       string `json:"description"`
	EnvironmentID     string `json:"environmentId"`
	DockerImage       string `json:"dockerImage"`
	DatabaseName      string `json:"databaseName"`
	DatabaseUser      string `json:"databaseUser"`
	DatabasePassword  string `json:"databasePassword"`
	ExternalPort      int    `json:"externalPort"`
	Env               string `json:"env"`
	ApplicationStatus string `json:"applicationStatus"`
}

// PostgresInput is the create/update payload.
type PostgresInput struct {
	Name             string `json:"name"`
	AppName          string `json:"appName,omitempty"`
	Description      string `json:"description,omitempty"`
	EnvironmentID    string `json:"environmentId,omitempty"`
	DockerImage      string `json:"dockerImage,omitempty"`
	DatabaseName     string `json:"databaseName,omitempty"`
	DatabaseUser     string `json:"databaseUser,omitempty"`
	DatabasePassword string `json:"databasePassword,omitempty"`
	ExternalPort     int    `json:"externalPort,omitempty"`
	Env              string `json:"env,omitempty"`
}

func (c *Client) CreatePostgres(ctx context.Context, in PostgresInput) (*Postgres, error) {
	var out Postgres
	if err := c.do(ctx, http.MethodPost, "postgres.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPostgres(ctx context.Context, id string) (*Postgres, error) {
	var out Postgres
	q := url.Values{"postgresId": {id}}
	if err := c.do(ctx, http.MethodGet, "postgres.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdatePostgres(ctx context.Context, id string, in PostgresInput) error {
	payload := struct {
		PostgresInput
		ID string `json:"postgresId"`
	}{PostgresInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "postgres.update", payload, nil, nil)
}

func (c *Client) DeletePostgres(ctx context.Context, id string) error {
	payload := map[string]string{"postgresId": id}
	return c.do(ctx, http.MethodPost, "postgres.remove", payload, nil, nil)
}

// DeployPostgres triggers an asynchronous deployment of an existing service.
func (c *Client) DeployPostgres(ctx context.Context, id string) error {
	payload := map[string]string{"postgresId": id}
	return c.do(ctx, http.MethodPost, "postgres.deploy", payload, nil, nil)
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Postgres -v`
Expected: PASS.

- [ ] **Step 5: Write failing acceptance test**

`internal/provider/postgres_resource_test.go`:

```go
package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-pg-proj-%d"
}

resource "dokploy_postgres" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-pg"
  docker_image   = %q
  database_name  = "app"
  database_user  = "app"
  # database_password omitted on purpose: provider must generate.
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
				Config: config("postgres:16"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_postgres.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_postgres.test", "app_name"),
					resource.TestCheckResourceAttr("dokploy_postgres.test", "status", "done"),
					resource.TestMatchResourceAttr("dokploy_postgres.test", "database_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
				),
			},
			{
				ResourceName:            "dokploy_postgres.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				Config: config("postgres:17"),
				Check:  resource.TestCheckResourceAttr("dokploy_postgres.test", "docker_image", "postgres:17"),
			},
		},
	})
}
```

- [ ] **Step 6: Run acceptance test to confirm it does not yet compile**

Run: `go build ./...`
Expected: FAIL — `undefined: NewPostgresResource`.

- [ ] **Step 7: Write `internal/provider/postgres_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &postgresResource{}
	_ resource.ResourceWithConfigure   = &postgresResource{}
	_ resource.ResourceWithImportState = &postgresResource{}
)

type postgresResource struct {
	client *client.Client
}

func NewPostgresResource() resource.Resource { return &postgresResource{} }

type postgresModel struct {
	ID               types.String   `tfsdk:"id"`
	EnvironmentID    types.String   `tfsdk:"environment_id"`
	Name             types.String   `tfsdk:"name"`
	Description      types.String   `tfsdk:"description"`
	DockerImage      types.String   `tfsdk:"docker_image"`
	ExternalPort     types.Int64    `tfsdk:"external_port"`
	Env              types.Map      `tfsdk:"env"`
	DatabaseName     types.String   `tfsdk:"database_name"`
	DatabaseUser     types.String   `tfsdk:"database_user"`
	DatabasePassword types.String   `tfsdk:"database_password"`
	AppName          types.String   `tfsdk:"app_name"`
	Status           types.String   `tfsdk:"status"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *postgresResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_postgres"
}

func (r *postgresResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy-managed PostgreSQL database service. Deployed and polled on apply.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Environment the database belongs to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description. Note: once set, removing this attribute does not clear it on the server.",
			},
			"docker_image": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "PostgreSQL image, e.g. `postgres:16`.",
			},
			"external_port": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Host port to expose the database on. Omit to keep internal-only.",
			},
			"env": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Extra environment variables.",
			},
			"database_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Initial database name.",
			},
			"database_user": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Database user.",
			},
			"database_password": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Database password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state. Changing this triggers a re-deploy, but only affects fresh containers — see the docs.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal service name (Dokploy-generated). Use this as the hostname from other services inside Dokploy's network.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of the most recent deploy.",
			},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *postgresResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *postgresResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan postgresModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	password := resolvePassword(plan.DatabasePassword)
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}

	pg, err := r.client.CreatePostgres(ctx, client.PostgresInput{
		Name:             plan.Name.ValueString(),
		AppName:          slugify(plan.Name.ValueString()),
		Description:      plan.Description.ValueString(),
		EnvironmentID:    plan.EnvironmentID.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabaseName:     plan.DatabaseName.ValueString(),
		DatabaseUser:     plan.DatabaseUser.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating postgres", err.Error())
		return
	}

	plan.ID = types.StringValue(pg.ID)
	plan.AppName = types.StringValue(pg.AppName)
	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error {
		return r.client.DeployPostgres(ctx, pg.ID)
	}
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetPostgres(ctx, pg.ID)
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Postgres deploy failed", err.Error())
		return
	}

	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *postgresResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state postgresModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pg, err := r.client.GetPostgres(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading postgres", err.Error())
		return
	}

	state.Name = types.StringValue(pg.Name)
	state.EnvironmentID = types.StringValue(pg.EnvironmentID)
	state.DockerImage = types.StringValue(pg.DockerImage)
	state.AppName = types.StringValue(pg.AppName)
	state.Status = types.StringValue(pg.ApplicationStatus)
	state.DatabaseName = types.StringValue(pg.DatabaseName)
	state.DatabaseUser = types.StringValue(pg.DatabaseUser)
	if pg.DatabasePassword != "" {
		state.DatabasePassword = types.StringValue(pg.DatabasePassword)
	}
	if pg.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(pg.Description)
	}
	if pg.ExternalPort != 0 || !state.ExternalPort.IsNull() {
		state.ExternalPort = types.Int64Value(int64(pg.ExternalPort))
	}
	if pg.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(pg.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *postgresResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan postgresModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state postgresModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve previously generated password if the user removed the attribute.
	password := plan.DatabasePassword.ValueString()
	if password == "" {
		password = state.DatabasePassword.ValueString()
	}
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}

	if err := r.client.UpdatePostgres(ctx, plan.ID.ValueString(), client.PostgresInput{
		Name:             plan.Name.ValueString(),
		Description:      plan.Description.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabaseName:     plan.DatabaseName.ValueString(),
		DatabaseUser:     plan.DatabaseUser.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating postgres", err.Error())
		return
	}

	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error {
		return r.client.DeployPostgres(ctx, plan.ID.ValueString())
	}
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetPostgres(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Postgres deploy failed", err.Error())
		return
	}

	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *postgresResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state postgresModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePostgres(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting postgres", err.Error())
	}
}

func (r *postgresResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Build, register in the provider, run acceptance test**

The resource must be added to the provider's `Resources()` list before `go build ./...` will compile fully. Open `internal/provider/provider.go` and append `NewPostgresResource,` to the slice returned by `Resources(_ context.Context) []func() resource.Resource`.

Run: `go build ./...` — expected: clean.
Run: `source .dokploy-test-env && TF_ACC=1 go test ./internal/provider/ -run TestAccPostgresResource -v -timeout 30m`
Expected: PASS (3 test steps including image upgrade and import).

- [ ] **Step 9: Commit**

```bash
gofmt -w .
git add internal/client/postgres.go internal/client/postgres_test.go internal/provider/postgres_resource.go internal/provider/postgres_resource_test.go internal/provider/provider.go
git commit -m "feat: dokploy_postgres resource"
```

---

## Task 4: MySQL resource

**Files:**
- Create: `internal/client/mysql.go`
- Create: `internal/client/mysql_test.go`
- Create: `internal/provider/mysql_resource.go`
- Create: `internal/provider/mysql_resource_test.go`
- Modify: `internal/provider/provider.go` (add `NewMysqlResource` to the resource list)

- [ ] **Step 1: Write failing client unit tests**

`internal/client/mysql_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Mysql -v` — Expected: FAIL.

- [ ] **Step 3: Write `internal/client/mysql.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Mysql is a Dokploy-managed MySQL service.
type Mysql struct {
	ID                   string `json:"mysqlId"`
	Name                 string `json:"name"`
	AppName              string `json:"appName"`
	Description          string `json:"description"`
	EnvironmentID        string `json:"environmentId"`
	DockerImage          string `json:"dockerImage"`
	DatabaseName         string `json:"databaseName"`
	DatabaseUser         string `json:"databaseUser"`
	DatabasePassword     string `json:"databasePassword"`
	DatabaseRootPassword string `json:"databaseRootPassword"`
	ExternalPort         int    `json:"externalPort"`
	Env                  string `json:"env"`
	ApplicationStatus    string `json:"applicationStatus"`
}

// MysqlInput is the create/update payload.
type MysqlInput struct {
	Name                 string `json:"name"`
	AppName              string `json:"appName,omitempty"`
	Description          string `json:"description,omitempty"`
	EnvironmentID        string `json:"environmentId,omitempty"`
	DockerImage          string `json:"dockerImage,omitempty"`
	DatabaseName         string `json:"databaseName,omitempty"`
	DatabaseUser         string `json:"databaseUser,omitempty"`
	DatabasePassword     string `json:"databasePassword,omitempty"`
	DatabaseRootPassword string `json:"databaseRootPassword,omitempty"`
	ExternalPort         int    `json:"externalPort,omitempty"`
	Env                  string `json:"env,omitempty"`
}

func (c *Client) CreateMysql(ctx context.Context, in MysqlInput) (*Mysql, error) {
	var out Mysql
	if err := c.do(ctx, http.MethodPost, "mysql.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMysql(ctx context.Context, id string) (*Mysql, error) {
	var out Mysql
	q := url.Values{"mysqlId": {id}}
	if err := c.do(ctx, http.MethodGet, "mysql.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMysql(ctx context.Context, id string, in MysqlInput) error {
	payload := struct {
		MysqlInput
		ID string `json:"mysqlId"`
	}{MysqlInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "mysql.update", payload, nil, nil)
}

func (c *Client) DeleteMysql(ctx context.Context, id string) error {
	payload := map[string]string{"mysqlId": id}
	return c.do(ctx, http.MethodPost, "mysql.remove", payload, nil, nil)
}

func (c *Client) DeployMysql(ctx context.Context, id string) error {
	payload := map[string]string{"mysqlId": id}
	return c.do(ctx, http.MethodPost, "mysql.deploy", payload, nil, nil)
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Mysql -v` — Expected: PASS.

- [ ] **Step 5: Write failing acceptance test**

`internal/provider/mysql_resource_test.go`:

```go
package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMysqlResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-my-proj-%d"
}

resource "dokploy_mysql" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-my"
  docker_image   = %q
  database_name  = "app"
  database_user  = "app"
  # database_password + database_root_password omitted on purpose.
  timeouts { create = "15m" update = "15m" }
}`, suffix, image)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("mysql:8"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mysql.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mysql.test", "status", "done"),
					resource.TestMatchResourceAttr("dokploy_mysql.test", "database_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
					resource.TestMatchResourceAttr("dokploy_mysql.test", "database_root_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
				),
			},
			{
				ResourceName:            "dokploy_mysql.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}
```

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewMysqlResource`.

- [ ] **Step 7: Write `internal/provider/mysql_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &mysqlResource{}
	_ resource.ResourceWithConfigure   = &mysqlResource{}
	_ resource.ResourceWithImportState = &mysqlResource{}
)

type mysqlResource struct{ client *client.Client }

func NewMysqlResource() resource.Resource { return &mysqlResource{} }

type mysqlModel struct {
	ID                   types.String   `tfsdk:"id"`
	EnvironmentID        types.String   `tfsdk:"environment_id"`
	Name                 types.String   `tfsdk:"name"`
	Description          types.String   `tfsdk:"description"`
	DockerImage          types.String   `tfsdk:"docker_image"`
	ExternalPort         types.Int64    `tfsdk:"external_port"`
	Env                  types.Map      `tfsdk:"env"`
	DatabaseName         types.String   `tfsdk:"database_name"`
	DatabaseUser         types.String   `tfsdk:"database_user"`
	DatabasePassword     types.String   `tfsdk:"database_password"`
	DatabaseRootPassword types.String   `tfsdk:"database_root_password"`
	AppName              types.String   `tfsdk:"app_name"`
	Status               types.String   `tfsdk:"status"`
	Timeouts             timeouts.Value `tfsdk:"timeouts"`
}

func (r *mysqlResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mysql"
}

func (r *mysqlResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy-managed MySQL database service. Deployed and polled on apply.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"environment_id": schema.StringAttribute{Required: true, MarkdownDescription: "Environment the database belongs to. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description":    schema.StringAttribute{Optional: true, MarkdownDescription: "Description. Once set, removing this attribute does not clear it on the server."},
			"docker_image":   schema.StringAttribute{Required: true, MarkdownDescription: "MySQL image, e.g. `mysql:8`."},
			"external_port":  schema.Int64Attribute{Optional: true, MarkdownDescription: "Host port to expose."},
			"env":            schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Extra environment variables."},
			"database_name":  schema.StringAttribute{Required: true, MarkdownDescription: "Initial database name."},
			"database_user":  schema.StringAttribute{Required: true, MarkdownDescription: "Database user."},
			"database_password": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				MarkdownDescription: "Database password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"database_root_password": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				MarkdownDescription: "MySQL root password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Internal service name (Dokploy-generated).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"status":   schema.StringAttribute{Computed: true, MarkdownDescription: "Status of the most recent deploy."},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *mysqlResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *mysqlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mysqlModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, diags := plan.Timeouts.Create(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	password := resolvePassword(plan.DatabasePassword)
	rootPassword := resolvePassword(plan.DatabaseRootPassword)
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}

	my, err := r.client.CreateMysql(ctx, client.MysqlInput{
		Name:                 plan.Name.ValueString(),
		AppName:              slugify(plan.Name.ValueString()),
		Description:          plan.Description.ValueString(),
		EnvironmentID:        plan.EnvironmentID.ValueString(),
		DockerImage:          plan.DockerImage.ValueString(),
		DatabaseName:         plan.DatabaseName.ValueString(),
		DatabaseUser:         plan.DatabaseUser.ValueString(),
		DatabasePassword:     password,
		DatabaseRootPassword: rootPassword,
		ExternalPort:         int(plan.ExternalPort.ValueInt64()),
		Env:                  envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating mysql", err.Error())
		return
	}
	plan.ID = types.StringValue(my.ID)
	plan.AppName = types.StringValue(my.AppName)
	plan.DatabasePassword = types.StringValue(password)
	plan.DatabaseRootPassword = types.StringValue(rootPassword)

	deployFn := func(ctx context.Context) error { return r.client.DeployMysql(ctx, my.ID) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMysql(ctx, my.ID)
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("MySQL deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mysqlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mysqlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	my, err := r.client.GetMysql(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading mysql", err.Error())
		return
	}
	state.Name = types.StringValue(my.Name)
	state.EnvironmentID = types.StringValue(my.EnvironmentID)
	state.DockerImage = types.StringValue(my.DockerImage)
	state.AppName = types.StringValue(my.AppName)
	state.Status = types.StringValue(my.ApplicationStatus)
	state.DatabaseName = types.StringValue(my.DatabaseName)
	state.DatabaseUser = types.StringValue(my.DatabaseUser)
	if my.DatabasePassword != "" {
		state.DatabasePassword = types.StringValue(my.DatabasePassword)
	}
	if my.DatabaseRootPassword != "" {
		state.DatabaseRootPassword = types.StringValue(my.DatabaseRootPassword)
	}
	if my.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(my.Description)
	}
	if my.ExternalPort != 0 || !state.ExternalPort.IsNull() {
		state.ExternalPort = types.Int64Value(int64(my.ExternalPort))
	}
	if my.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(my.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *mysqlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mysqlModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state mysqlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	password := plan.DatabasePassword.ValueString()
	if password == "" {
		password = state.DatabasePassword.ValueString()
	}
	rootPassword := plan.DatabaseRootPassword.ValueString()
	if rootPassword == "" {
		rootPassword = state.DatabaseRootPassword.ValueString()
	}
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	if err := r.client.UpdateMysql(ctx, plan.ID.ValueString(), client.MysqlInput{
		Name:                 plan.Name.ValueString(),
		Description:          plan.Description.ValueString(),
		DockerImage:          plan.DockerImage.ValueString(),
		DatabaseName:         plan.DatabaseName.ValueString(),
		DatabaseUser:         plan.DatabaseUser.ValueString(),
		DatabasePassword:     password,
		DatabaseRootPassword: rootPassword,
		ExternalPort:         int(plan.ExternalPort.ValueInt64()),
		Env:                  envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating mysql", err.Error())
		return
	}
	plan.DatabasePassword = types.StringValue(password)
	plan.DatabaseRootPassword = types.StringValue(rootPassword)

	deployFn := func(ctx context.Context) error { return r.client.DeployMysql(ctx, plan.ID.ValueString()) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMysql(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("MySQL deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mysqlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mysqlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMysql(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting mysql", err.Error())
	}
}

func (r *mysqlResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run acceptance test**

Append `NewMysqlResource,` to `Resources()` in `internal/provider/provider.go`.

Run: `go build ./...` — Expected: clean.
Run: `source .dokploy-test-env && TF_ACC=1 go test ./internal/provider/ -run TestAccMysqlResource -v -timeout 30m` — Expected: PASS.

- [ ] **Step 9: Commit**

```bash
gofmt -w .
git add internal/client/mysql.go internal/client/mysql_test.go internal/provider/mysql_resource.go internal/provider/mysql_resource_test.go internal/provider/provider.go
git commit -m "feat: dokploy_mysql resource"
```

---

## Task 5: MariaDB resource

MariaDB's API mirrors MySQL's (same field set including `databaseRootPassword`), just on the `mariadb.*` router. The plan repeats the full code rather than referencing Task 4 — this protects the reader who jumps in here without context.

**Files:**
- Create: `internal/client/mariadb.go`
- Create: `internal/client/mariadb_test.go`
- Create: `internal/provider/mariadb_resource.go`
- Create: `internal/provider/mariadb_resource_test.go`
- Modify: `internal/provider/provider.go` (add `NewMariadbResource`)

- [ ] **Step 1: Write failing client unit tests**

`internal/client/mariadb_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateMariadb(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mariadb.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body MariadbInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DatabaseRootPassword != "rootpw" {
			t.Errorf("databaseRootPassword = %q", body.DatabaseRootPassword)
		}
		_ = json.NewEncoder(w).Encode(Mariadb{ID: "ma1", AppName: "db-abc"})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	ma, err := c.CreateMariadb(context.Background(), MariadbInput{
		Name: "db", AppName: "db", EnvironmentID: "env",
		DockerImage:          "mariadb:11",
		DatabaseName:         "app",
		DatabaseUser:         "app",
		DatabasePassword:     "pw",
		DatabaseRootPassword: "rootpw",
	})
	if err != nil {
		t.Fatalf("CreateMariadb() error = %v", err)
	}
	if ma.ID != "ma1" {
		t.Errorf("ma = %+v", ma)
	}
}

func TestGetMariadb_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetMariadb(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Mariadb -v` — Expected: FAIL.

- [ ] **Step 3: Write `internal/client/mariadb.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

type Mariadb struct {
	ID                   string `json:"mariadbId"`
	Name                 string `json:"name"`
	AppName              string `json:"appName"`
	Description          string `json:"description"`
	EnvironmentID        string `json:"environmentId"`
	DockerImage          string `json:"dockerImage"`
	DatabaseName         string `json:"databaseName"`
	DatabaseUser         string `json:"databaseUser"`
	DatabasePassword     string `json:"databasePassword"`
	DatabaseRootPassword string `json:"databaseRootPassword"`
	ExternalPort         int    `json:"externalPort"`
	Env                  string `json:"env"`
	ApplicationStatus    string `json:"applicationStatus"`
}

type MariadbInput struct {
	Name                 string `json:"name"`
	AppName              string `json:"appName,omitempty"`
	Description          string `json:"description,omitempty"`
	EnvironmentID        string `json:"environmentId,omitempty"`
	DockerImage          string `json:"dockerImage,omitempty"`
	DatabaseName         string `json:"databaseName,omitempty"`
	DatabaseUser         string `json:"databaseUser,omitempty"`
	DatabasePassword     string `json:"databasePassword,omitempty"`
	DatabaseRootPassword string `json:"databaseRootPassword,omitempty"`
	ExternalPort         int    `json:"externalPort,omitempty"`
	Env                  string `json:"env,omitempty"`
}

func (c *Client) CreateMariadb(ctx context.Context, in MariadbInput) (*Mariadb, error) {
	var out Mariadb
	if err := c.do(ctx, http.MethodPost, "mariadb.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMariadb(ctx context.Context, id string) (*Mariadb, error) {
	var out Mariadb
	q := url.Values{"mariadbId": {id}}
	if err := c.do(ctx, http.MethodGet, "mariadb.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMariadb(ctx context.Context, id string, in MariadbInput) error {
	payload := struct {
		MariadbInput
		ID string `json:"mariadbId"`
	}{MariadbInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "mariadb.update", payload, nil, nil)
}

func (c *Client) DeleteMariadb(ctx context.Context, id string) error {
	payload := map[string]string{"mariadbId": id}
	return c.do(ctx, http.MethodPost, "mariadb.remove", payload, nil, nil)
}

func (c *Client) DeployMariadb(ctx context.Context, id string) error {
	payload := map[string]string{"mariadbId": id}
	return c.do(ctx, http.MethodPost, "mariadb.deploy", payload, nil, nil)
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Mariadb -v` — Expected: PASS.

- [ ] **Step 5: Write failing acceptance test**

`internal/provider/mariadb_resource_test.go`:

```go
package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMariadbResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-ma-proj-%d"
}

resource "dokploy_mariadb" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-ma"
  docker_image   = %q
  database_name  = "app"
  database_user  = "app"
  timeouts { create = "15m" update = "15m" }
}`, suffix, image)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("mariadb:11"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mariadb.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mariadb.test", "status", "done"),
					resource.TestMatchResourceAttr("dokploy_mariadb.test", "database_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
					resource.TestMatchResourceAttr("dokploy_mariadb.test", "database_root_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
				),
			},
			{
				ResourceName:            "dokploy_mariadb.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}
```

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewMariadbResource`.

- [ ] **Step 7: Write `internal/provider/mariadb_resource.go`**

Same structure as the MySQL resource, on the `mariadb.*` client methods.

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &mariadbResource{}
	_ resource.ResourceWithConfigure   = &mariadbResource{}
	_ resource.ResourceWithImportState = &mariadbResource{}
)

type mariadbResource struct{ client *client.Client }

func NewMariadbResource() resource.Resource { return &mariadbResource{} }

type mariadbModel struct {
	ID                   types.String   `tfsdk:"id"`
	EnvironmentID        types.String   `tfsdk:"environment_id"`
	Name                 types.String   `tfsdk:"name"`
	Description          types.String   `tfsdk:"description"`
	DockerImage          types.String   `tfsdk:"docker_image"`
	ExternalPort         types.Int64    `tfsdk:"external_port"`
	Env                  types.Map      `tfsdk:"env"`
	DatabaseName         types.String   `tfsdk:"database_name"`
	DatabaseUser         types.String   `tfsdk:"database_user"`
	DatabasePassword     types.String   `tfsdk:"database_password"`
	DatabaseRootPassword types.String   `tfsdk:"database_root_password"`
	AppName              types.String   `tfsdk:"app_name"`
	Status               types.String   `tfsdk:"status"`
	Timeouts             timeouts.Value `tfsdk:"timeouts"`
}

func (r *mariadbResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mariadb"
}

func (r *mariadbResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy-managed MariaDB database service. Deployed and polled on apply.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"environment_id": schema.StringAttribute{Required: true, MarkdownDescription: "Environment the database belongs to. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description":    schema.StringAttribute{Optional: true, MarkdownDescription: "Description. Once set, removing this attribute does not clear it on the server."},
			"docker_image":   schema.StringAttribute{Required: true, MarkdownDescription: "MariaDB image, e.g. `mariadb:11`."},
			"external_port":  schema.Int64Attribute{Optional: true, MarkdownDescription: "Host port to expose."},
			"env":            schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Extra environment variables."},
			"database_name":  schema.StringAttribute{Required: true, MarkdownDescription: "Initial database name."},
			"database_user":  schema.StringAttribute{Required: true, MarkdownDescription: "Database user."},
			"database_password": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				MarkdownDescription: "Database password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"database_root_password": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				MarkdownDescription: "MariaDB root password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Internal service name (Dokploy-generated).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"status":   schema.StringAttribute{Computed: true, MarkdownDescription: "Status of the most recent deploy."},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *mariadbResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *mariadbResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mariadbModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, diags := plan.Timeouts.Create(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	password := resolvePassword(plan.DatabasePassword)
	rootPassword := resolvePassword(plan.DatabaseRootPassword)
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	ma, err := r.client.CreateMariadb(ctx, client.MariadbInput{
		Name:                 plan.Name.ValueString(),
		AppName:              slugify(plan.Name.ValueString()),
		Description:          plan.Description.ValueString(),
		EnvironmentID:        plan.EnvironmentID.ValueString(),
		DockerImage:          plan.DockerImage.ValueString(),
		DatabaseName:         plan.DatabaseName.ValueString(),
		DatabaseUser:         plan.DatabaseUser.ValueString(),
		DatabasePassword:     password,
		DatabaseRootPassword: rootPassword,
		ExternalPort:         int(plan.ExternalPort.ValueInt64()),
		Env:                  envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating mariadb", err.Error())
		return
	}
	plan.ID = types.StringValue(ma.ID)
	plan.AppName = types.StringValue(ma.AppName)
	plan.DatabasePassword = types.StringValue(password)
	plan.DatabaseRootPassword = types.StringValue(rootPassword)

	deployFn := func(ctx context.Context) error { return r.client.DeployMariadb(ctx, ma.ID) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMariadb(ctx, ma.ID)
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("MariaDB deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mariadbResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mariadbModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ma, err := r.client.GetMariadb(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading mariadb", err.Error())
		return
	}
	state.Name = types.StringValue(ma.Name)
	state.EnvironmentID = types.StringValue(ma.EnvironmentID)
	state.DockerImage = types.StringValue(ma.DockerImage)
	state.AppName = types.StringValue(ma.AppName)
	state.Status = types.StringValue(ma.ApplicationStatus)
	state.DatabaseName = types.StringValue(ma.DatabaseName)
	state.DatabaseUser = types.StringValue(ma.DatabaseUser)
	if ma.DatabasePassword != "" {
		state.DatabasePassword = types.StringValue(ma.DatabasePassword)
	}
	if ma.DatabaseRootPassword != "" {
		state.DatabaseRootPassword = types.StringValue(ma.DatabaseRootPassword)
	}
	if ma.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(ma.Description)
	}
	if ma.ExternalPort != 0 || !state.ExternalPort.IsNull() {
		state.ExternalPort = types.Int64Value(int64(ma.ExternalPort))
	}
	if ma.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(ma.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *mariadbResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mariadbModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state mariadbModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	password := plan.DatabasePassword.ValueString()
	if password == "" {
		password = state.DatabasePassword.ValueString()
	}
	rootPassword := plan.DatabaseRootPassword.ValueString()
	if rootPassword == "" {
		rootPassword = state.DatabaseRootPassword.ValueString()
	}
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	if err := r.client.UpdateMariadb(ctx, plan.ID.ValueString(), client.MariadbInput{
		Name:                 plan.Name.ValueString(),
		Description:          plan.Description.ValueString(),
		DockerImage:          plan.DockerImage.ValueString(),
		DatabaseName:         plan.DatabaseName.ValueString(),
		DatabaseUser:         plan.DatabaseUser.ValueString(),
		DatabasePassword:     password,
		DatabaseRootPassword: rootPassword,
		ExternalPort:         int(plan.ExternalPort.ValueInt64()),
		Env:                  envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating mariadb", err.Error())
		return
	}
	plan.DatabasePassword = types.StringValue(password)
	plan.DatabaseRootPassword = types.StringValue(rootPassword)

	deployFn := func(ctx context.Context) error { return r.client.DeployMariadb(ctx, plan.ID.ValueString()) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMariadb(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("MariaDB deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mariadbResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mariadbModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMariadb(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting mariadb", err.Error())
	}
}

func (r *mariadbResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run acceptance test**

Append `NewMariadbResource,` to `Resources()`. Build, then:
`source .dokploy-test-env && TF_ACC=1 go test ./internal/provider/ -run TestAccMariadbResource -v -timeout 30m` — Expected: PASS.

- [ ] **Step 9: Commit**

```bash
gofmt -w .
git add internal/client/mariadb.go internal/client/mariadb_test.go internal/provider/mariadb_resource.go internal/provider/mariadb_resource_test.go internal/provider/provider.go
git commit -m "feat: dokploy_mariadb resource"
```

---

## Task 6: MongoDB resource

Mongo has user/password but no `database_name` / no root-password (the user IS the root).

**Files:**
- Create: `internal/client/mongo.go`
- Create: `internal/client/mongo_test.go`
- Create: `internal/provider/mongo_resource.go`
- Create: `internal/provider/mongo_resource_test.go`
- Modify: `internal/provider/provider.go`

- [ ] **Step 1: Write failing client unit tests**

`internal/client/mongo_test.go`:

```go
package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateMongo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mongo.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"mongoId":"mo1","appName":"db-abc","databaseUser":"root"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	mo, err := c.CreateMongo(context.Background(), MongoInput{
		Name: "db", AppName: "db", EnvironmentID: "env",
		DockerImage:      "mongo:7",
		DatabaseUser:     "root",
		DatabasePassword: "pw",
	})
	if err != nil {
		t.Fatalf("CreateMongo() error = %v", err)
	}
	if mo.ID != "mo1" {
		t.Errorf("mo = %+v", mo)
	}
}

func TestGetMongo_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetMongo(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Mongo -v` — Expected: FAIL.

- [ ] **Step 3: Write `internal/client/mongo.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

type Mongo struct {
	ID                string `json:"mongoId"`
	Name              string `json:"name"`
	AppName           string `json:"appName"`
	Description       string `json:"description"`
	EnvironmentID     string `json:"environmentId"`
	DockerImage       string `json:"dockerImage"`
	DatabaseUser      string `json:"databaseUser"`
	DatabasePassword  string `json:"databasePassword"`
	ExternalPort      int    `json:"externalPort"`
	Env               string `json:"env"`
	ApplicationStatus string `json:"applicationStatus"`
}

type MongoInput struct {
	Name             string `json:"name"`
	AppName          string `json:"appName,omitempty"`
	Description      string `json:"description,omitempty"`
	EnvironmentID    string `json:"environmentId,omitempty"`
	DockerImage      string `json:"dockerImage,omitempty"`
	DatabaseUser     string `json:"databaseUser,omitempty"`
	DatabasePassword string `json:"databasePassword,omitempty"`
	ExternalPort     int    `json:"externalPort,omitempty"`
	Env              string `json:"env,omitempty"`
}

func (c *Client) CreateMongo(ctx context.Context, in MongoInput) (*Mongo, error) {
	var out Mongo
	if err := c.do(ctx, http.MethodPost, "mongo.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMongo(ctx context.Context, id string) (*Mongo, error) {
	var out Mongo
	q := url.Values{"mongoId": {id}}
	if err := c.do(ctx, http.MethodGet, "mongo.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMongo(ctx context.Context, id string, in MongoInput) error {
	payload := struct {
		MongoInput
		ID string `json:"mongoId"`
	}{MongoInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "mongo.update", payload, nil, nil)
}

func (c *Client) DeleteMongo(ctx context.Context, id string) error {
	payload := map[string]string{"mongoId": id}
	return c.do(ctx, http.MethodPost, "mongo.remove", payload, nil, nil)
}

func (c *Client) DeployMongo(ctx context.Context, id string) error {
	payload := map[string]string{"mongoId": id}
	return c.do(ctx, http.MethodPost, "mongo.deploy", payload, nil, nil)
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Mongo -v` — Expected: PASS.

- [ ] **Step 5: Write failing acceptance test**

`internal/provider/mongo_resource_test.go`:

```go
package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMongoResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-mo-proj-%d"
}

resource "dokploy_mongo" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-mo"
  docker_image   = %q
  database_user  = "root"
  timeouts { create = "15m" update = "15m" }
}`, suffix, image)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("mongo:7"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mongo.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mongo.test", "status", "done"),
					resource.TestMatchResourceAttr("dokploy_mongo.test", "database_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
				),
			},
			{
				ResourceName:            "dokploy_mongo.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}
```

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewMongoResource`.

- [ ] **Step 7: Write `internal/provider/mongo_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &mongoResource{}
	_ resource.ResourceWithConfigure   = &mongoResource{}
	_ resource.ResourceWithImportState = &mongoResource{}
)

type mongoResource struct{ client *client.Client }

func NewMongoResource() resource.Resource { return &mongoResource{} }

type mongoModel struct {
	ID               types.String   `tfsdk:"id"`
	EnvironmentID    types.String   `tfsdk:"environment_id"`
	Name             types.String   `tfsdk:"name"`
	Description      types.String   `tfsdk:"description"`
	DockerImage      types.String   `tfsdk:"docker_image"`
	ExternalPort     types.Int64    `tfsdk:"external_port"`
	Env              types.Map      `tfsdk:"env"`
	DatabaseUser     types.String   `tfsdk:"database_user"`
	DatabasePassword types.String   `tfsdk:"database_password"`
	AppName          types.String   `tfsdk:"app_name"`
	Status           types.String   `tfsdk:"status"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *mongoResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mongo"
}

func (r *mongoResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy-managed MongoDB service. Deployed and polled on apply.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"environment_id": schema.StringAttribute{Required: true, MarkdownDescription: "Environment the database belongs to. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description":    schema.StringAttribute{Optional: true, MarkdownDescription: "Description. Once set, removing this attribute does not clear it on the server."},
			"docker_image":   schema.StringAttribute{Required: true, MarkdownDescription: "MongoDB image, e.g. `mongo:7`."},
			"external_port":  schema.Int64Attribute{Optional: true, MarkdownDescription: "Host port to expose."},
			"env":            schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Extra environment variables."},
			"database_user":  schema.StringAttribute{Required: true, MarkdownDescription: "Root user name."},
			"database_password": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				MarkdownDescription: "Root password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Internal service name (Dokploy-generated).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"status":   schema.StringAttribute{Computed: true, MarkdownDescription: "Status of the most recent deploy."},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *mongoResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *mongoResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mongoModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, diags := plan.Timeouts.Create(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	password := resolvePassword(plan.DatabasePassword)
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	mo, err := r.client.CreateMongo(ctx, client.MongoInput{
		Name:             plan.Name.ValueString(),
		AppName:          slugify(plan.Name.ValueString()),
		Description:      plan.Description.ValueString(),
		EnvironmentID:    plan.EnvironmentID.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabaseUser:     plan.DatabaseUser.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating mongo", err.Error())
		return
	}
	plan.ID = types.StringValue(mo.ID)
	plan.AppName = types.StringValue(mo.AppName)
	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error { return r.client.DeployMongo(ctx, mo.ID) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMongo(ctx, mo.ID)
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Mongo deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mongoResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mongoModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mo, err := r.client.GetMongo(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading mongo", err.Error())
		return
	}
	state.Name = types.StringValue(mo.Name)
	state.EnvironmentID = types.StringValue(mo.EnvironmentID)
	state.DockerImage = types.StringValue(mo.DockerImage)
	state.AppName = types.StringValue(mo.AppName)
	state.Status = types.StringValue(mo.ApplicationStatus)
	state.DatabaseUser = types.StringValue(mo.DatabaseUser)
	if mo.DatabasePassword != "" {
		state.DatabasePassword = types.StringValue(mo.DatabasePassword)
	}
	if mo.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(mo.Description)
	}
	if mo.ExternalPort != 0 || !state.ExternalPort.IsNull() {
		state.ExternalPort = types.Int64Value(int64(mo.ExternalPort))
	}
	if mo.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(mo.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *mongoResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mongoModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state mongoModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	password := plan.DatabasePassword.ValueString()
	if password == "" {
		password = state.DatabasePassword.ValueString()
	}
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	if err := r.client.UpdateMongo(ctx, plan.ID.ValueString(), client.MongoInput{
		Name:             plan.Name.ValueString(),
		Description:      plan.Description.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabaseUser:     plan.DatabaseUser.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating mongo", err.Error())
		return
	}
	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error { return r.client.DeployMongo(ctx, plan.ID.ValueString()) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMongo(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Mongo deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mongoResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mongoModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMongo(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting mongo", err.Error())
	}
}

func (r *mongoResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run acceptance test**

Append `NewMongoResource,` to `Resources()`. Build, then:
`source .dokploy-test-env && TF_ACC=1 go test ./internal/provider/ -run TestAccMongoResource -v -timeout 30m` — Expected: PASS.

- [ ] **Step 9: Commit**

```bash
gofmt -w .
git add internal/client/mongo.go internal/client/mongo_test.go internal/provider/mongo_resource.go internal/provider/mongo_resource_test.go internal/provider/provider.go
git commit -m "feat: dokploy_mongo resource"
```

---

## Task 7: Redis resource

Redis has only `databasePassword` — no user, no database name.

**Files:**
- Create: `internal/client/redis.go`
- Create: `internal/client/redis_test.go`
- Create: `internal/provider/redis_resource.go`
- Create: `internal/provider/redis_resource_test.go`
- Modify: `internal/provider/provider.go`

- [ ] **Step 1: Write failing client unit tests**

`internal/client/redis_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Redis -v` — Expected: FAIL.

- [ ] **Step 3: Write `internal/client/redis.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

type Redis struct {
	ID                string `json:"redisId"`
	Name              string `json:"name"`
	AppName           string `json:"appName"`
	Description       string `json:"description"`
	EnvironmentID     string `json:"environmentId"`
	DockerImage       string `json:"dockerImage"`
	DatabasePassword  string `json:"databasePassword"`
	ExternalPort      int    `json:"externalPort"`
	Env               string `json:"env"`
	ApplicationStatus string `json:"applicationStatus"`
}

type RedisInput struct {
	Name             string `json:"name"`
	AppName          string `json:"appName,omitempty"`
	Description      string `json:"description,omitempty"`
	EnvironmentID    string `json:"environmentId,omitempty"`
	DockerImage      string `json:"dockerImage,omitempty"`
	DatabasePassword string `json:"databasePassword,omitempty"`
	ExternalPort     int    `json:"externalPort,omitempty"`
	Env              string `json:"env,omitempty"`
}

func (c *Client) CreateRedis(ctx context.Context, in RedisInput) (*Redis, error) {
	var out Redis
	if err := c.do(ctx, http.MethodPost, "redis.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRedis(ctx context.Context, id string) (*Redis, error) {
	var out Redis
	q := url.Values{"redisId": {id}}
	if err := c.do(ctx, http.MethodGet, "redis.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateRedis(ctx context.Context, id string, in RedisInput) error {
	payload := struct {
		RedisInput
		ID string `json:"redisId"`
	}{RedisInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "redis.update", payload, nil, nil)
}

func (c *Client) DeleteRedis(ctx context.Context, id string) error {
	payload := map[string]string{"redisId": id}
	return c.do(ctx, http.MethodPost, "redis.remove", payload, nil, nil)
}

func (c *Client) DeployRedis(ctx context.Context, id string) error {
	payload := map[string]string{"redisId": id}
	return c.do(ctx, http.MethodPost, "redis.deploy", payload, nil, nil)
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Redis -v` — Expected: PASS.

- [ ] **Step 5: Write failing acceptance test**

`internal/provider/redis_resource_test.go`:

```go
package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRedisResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-re-proj-%d"
}

resource "dokploy_redis" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-re"
  docker_image   = %q
  timeouts { create = "15m" update = "15m" }
}`, suffix, image)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("redis:7.2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_redis.test", "id"),
					resource.TestCheckResourceAttr("dokploy_redis.test", "status", "done"),
					resource.TestMatchResourceAttr("dokploy_redis.test", "database_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
				),
			},
			{
				ResourceName:            "dokploy_redis.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}
```

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewRedisResource`.

- [ ] **Step 7: Write `internal/provider/redis_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &redisResource{}
	_ resource.ResourceWithConfigure   = &redisResource{}
	_ resource.ResourceWithImportState = &redisResource{}
)

type redisResource struct{ client *client.Client }

func NewRedisResource() resource.Resource { return &redisResource{} }

type redisModel struct {
	ID               types.String   `tfsdk:"id"`
	EnvironmentID    types.String   `tfsdk:"environment_id"`
	Name             types.String   `tfsdk:"name"`
	Description      types.String   `tfsdk:"description"`
	DockerImage      types.String   `tfsdk:"docker_image"`
	ExternalPort     types.Int64    `tfsdk:"external_port"`
	Env              types.Map      `tfsdk:"env"`
	DatabasePassword types.String   `tfsdk:"database_password"`
	AppName          types.String   `tfsdk:"app_name"`
	Status           types.String   `tfsdk:"status"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *redisResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_redis"
}

func (r *redisResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy-managed Redis service. Deployed and polled on apply.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"environment_id": schema.StringAttribute{Required: true, MarkdownDescription: "Environment the cache belongs to. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description":    schema.StringAttribute{Optional: true, MarkdownDescription: "Description. Once set, removing this attribute does not clear it on the server."},
			"docker_image":   schema.StringAttribute{Required: true, MarkdownDescription: "Redis image, e.g. `redis:7.2`."},
			"external_port":  schema.Int64Attribute{Optional: true, MarkdownDescription: "Host port to expose."},
			"env":            schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Extra environment variables."},
			"database_password": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				MarkdownDescription: "Redis `requirepass` value. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Internal service name (Dokploy-generated).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"status":   schema.StringAttribute{Computed: true, MarkdownDescription: "Status of the most recent deploy."},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *redisResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *redisResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan redisModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, diags := plan.Timeouts.Create(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	password := resolvePassword(plan.DatabasePassword)
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	re, err := r.client.CreateRedis(ctx, client.RedisInput{
		Name:             plan.Name.ValueString(),
		AppName:          slugify(plan.Name.ValueString()),
		Description:      plan.Description.ValueString(),
		EnvironmentID:    plan.EnvironmentID.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating redis", err.Error())
		return
	}
	plan.ID = types.StringValue(re.ID)
	plan.AppName = types.StringValue(re.AppName)
	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error { return r.client.DeployRedis(ctx, re.ID) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetRedis(ctx, re.ID)
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Redis deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *redisResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state redisModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	re, err := r.client.GetRedis(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading redis", err.Error())
		return
	}
	state.Name = types.StringValue(re.Name)
	state.EnvironmentID = types.StringValue(re.EnvironmentID)
	state.DockerImage = types.StringValue(re.DockerImage)
	state.AppName = types.StringValue(re.AppName)
	state.Status = types.StringValue(re.ApplicationStatus)
	if re.DatabasePassword != "" {
		state.DatabasePassword = types.StringValue(re.DatabasePassword)
	}
	if re.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(re.Description)
	}
	if re.ExternalPort != 0 || !state.ExternalPort.IsNull() {
		state.ExternalPort = types.Int64Value(int64(re.ExternalPort))
	}
	if re.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(re.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *redisResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan redisModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state redisModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	password := plan.DatabasePassword.ValueString()
	if password == "" {
		password = state.DatabasePassword.ValueString()
	}
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	if err := r.client.UpdateRedis(ctx, plan.ID.ValueString(), client.RedisInput{
		Name:             plan.Name.ValueString(),
		Description:      plan.Description.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating redis", err.Error())
		return
	}
	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error { return r.client.DeployRedis(ctx, plan.ID.ValueString()) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetRedis(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Redis deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *redisResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state redisModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteRedis(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting redis", err.Error())
	}
}

func (r *redisResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run acceptance test**

Append `NewRedisResource,` to `Resources()`. Build, then:
`source .dokploy-test-env && TF_ACC=1 go test ./internal/provider/ -run TestAccRedisResource -v -timeout 30m` — Expected: PASS.

- [ ] **Step 9: Commit**

```bash
gofmt -w .
git add internal/client/redis.go internal/client/redis_test.go internal/provider/redis_resource.go internal/provider/redis_resource_test.go internal/provider/provider.go
git commit -m "feat: dokploy_redis resource"
```

---

## Task 8: Examples, README, and generated docs

**Files:**
- Create: `examples/resources/dokploy_postgres/resource.tf`
- Create: `examples/resources/dokploy_postgres/import.sh`
- Create: `examples/resources/dokploy_mysql/resource.tf`
- Create: `examples/resources/dokploy_mysql/import.sh`
- Create: `examples/resources/dokploy_mariadb/resource.tf`
- Create: `examples/resources/dokploy_mariadb/import.sh`
- Create: `examples/resources/dokploy_mongo/resource.tf`
- Create: `examples/resources/dokploy_mongo/import.sh`
- Create: `examples/resources/dokploy_redis/resource.tf`
- Create: `examples/resources/dokploy_redis/import.sh`
- Modify: `README.md` (add the 5 lines under Resources)
- Regenerated: `docs/resources/*.md` (via `go generate ./...`)

- [ ] **Step 1: Create example files**

`examples/resources/dokploy_postgres/resource.tf`:

```hcl
resource "dokploy_postgres" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "postgres:16"
  database_name  = "app"
  database_user  = "app"
  # database_password omitted → provider generates a 32-char random.
}
```

`examples/resources/dokploy_postgres/import.sh`:

```bash
terraform import dokploy_postgres.db <postgresId>
```

`examples/resources/dokploy_mysql/resource.tf`:

```hcl
resource "dokploy_mysql" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "mysql:8"
  database_name  = "app"
  database_user  = "app"
  # database_password and database_root_password omitted → provider generates.
}
```

`examples/resources/dokploy_mysql/import.sh`:

```bash
terraform import dokploy_mysql.db <mysqlId>
```

`examples/resources/dokploy_mariadb/resource.tf`:

```hcl
resource "dokploy_mariadb" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "mariadb:11"
  database_name  = "app"
  database_user  = "app"
}
```

`examples/resources/dokploy_mariadb/import.sh`:

```bash
terraform import dokploy_mariadb.db <mariadbId>
```

`examples/resources/dokploy_mongo/resource.tf`:

```hcl
resource "dokploy_mongo" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "mongo:7"
  database_user  = "root"
}
```

`examples/resources/dokploy_mongo/import.sh`:

```bash
terraform import dokploy_mongo.db <mongoId>
```

`examples/resources/dokploy_redis/resource.tf`:

```hcl
resource "dokploy_redis" "cache" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-cache"
  docker_image   = "redis:7.2"
}
```

`examples/resources/dokploy_redis/import.sh`:

```bash
terraform import dokploy_redis.cache <redisId>
```

- [ ] **Step 2: Update `README.md`**

Find the line `- \`dokploy_domain\` — domains routing traffic to applications` in the `## Resources` section of `README.md` and insert the five new lines directly after it:

```markdown
- `dokploy_postgres` — managed PostgreSQL service
- `dokploy_mysql` — managed MySQL service
- `dokploy_mariadb` — managed MariaDB service
- `dokploy_mongo` — managed MongoDB service
- `dokploy_redis` — managed Redis service
```

- [ ] **Step 3: Regenerate documentation**

Run: `go generate ./...`
Expected: new files appear under `docs/resources/` for each database.

- [ ] **Step 4: Verify**

Run: `git status --short`
Expected: example files are new (`?? examples/...`), README and `docs/resources/*.md` are modified (`M README.md`, `M docs/resources/*`).

- [ ] **Step 5: Commit**

```bash
gofmt -w .
git add examples README.md docs/
git commit -m "docs: examples, README entries, and generated docs for v0.2 databases"
```

---

## Task 9: Release v0.2.0

**Files:** none (release lives in tags + GoReleaser-built GitHub Release).

- [ ] **Step 1: Final verification**

Run from the repo root:

```bash
go build ./...
go vet ./...
go test ./internal/client/... -v
go generate ./...
git status --short        # expect: clean (no uncommitted docs diff)
```

Expected: all green. If `go generate` produces a diff, commit it under `docs: regenerate docs` before continuing.

Run the acceptance suite end-to-end one more time:

```bash
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/... -v -timeout 60m
```

Expected: all tests PASS (the v0.1 resource + e2e tests, plus the 5 new DB tests). After it finishes, confirm the live Dokploy instance has no leftover `tf-acc-*` resources.

- [ ] **Step 2: Push the merge to master**

If you have been working on a feature branch, merge it back to master (the v0.1 finishing flow):

```bash
git checkout master
git pull --ff-only
git merge --ff-only <feature-branch>
git push origin master
```

If you have been on master directly: `git push origin master`.

- [ ] **Step 3: Tag v0.2.0 and push**

```bash
git tag v0.2.0
git push origin v0.2.0
```

The `Release` workflow (`.github/workflows/release.yml`) triggers automatically. It imports the GPG key, runs GoReleaser, and publishes a GitHub Release with signed binaries + `SHA256SUMS` + `manifest.json`.

- [ ] **Step 4: Watch the workflow**

```bash
RUN_ID=$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')
gh run watch "$RUN_ID" --exit-status
```

Expected: SUCCESS in ~4 minutes.

- [ ] **Step 5: Verify the GitHub Release**

```bash
gh release view v0.2.0 --json assets -q '.assets[].name' | sort
```

Expected output (13 assets — same shape as v0.1.0):

```
terraform-provider-dokploy_0.2.0_SHA256SUMS
terraform-provider-dokploy_0.2.0_SHA256SUMS.sig
terraform-provider-dokploy_0.2.0_darwin_amd64.zip
terraform-provider-dokploy_0.2.0_darwin_arm64.zip
terraform-provider-dokploy_0.2.0_freebsd_amd64.zip
terraform-provider-dokploy_0.2.0_freebsd_arm.zip
terraform-provider-dokploy_0.2.0_freebsd_arm64.zip
terraform-provider-dokploy_0.2.0_linux_amd64.zip
terraform-provider-dokploy_0.2.0_linux_arm.zip
terraform-provider-dokploy_0.2.0_linux_arm64.zip
terraform-provider-dokploy_0.2.0_manifest.json
terraform-provider-dokploy_0.2.0_windows_amd64.zip
terraform-provider-dokploy_0.2.0_windows_arm64.zip
```

Confirm `SHA256SUMS` contains the manifest line:

```bash
gh release download v0.2.0 -p '*_SHA256SUMS' -O - | grep manifest
```

Expected: one line ending in `terraform-provider-dokploy_0.2.0_manifest.json`.

- [ ] **Step 6: Confirm Registry picked up the new version**

Within ~5 min of the workflow finishing, the Terraform Registry detects the release. Verify:

```bash
/usr/bin/curl -s "https://registry.terraform.io/v1/providers/lucasaarch/dokploy/versions" | python3 -m json.tool | grep -E '"version"' | head -5
```

Expected: a `"version": "0.2.0"` entry alongside `0.1.0`.

The provider is available at `https://registry.terraform.io/providers/lucasaarch/dokploy/0.2.0`.

---

## Self-review checklist

Before considering this plan complete, the implementer should confirm:

- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` clean
- [ ] `go test ./internal/client/... -v` — all unit tests pass (existing + new)
- [ ] `go generate ./...` produces no uncommitted diff
- [ ] All 5 new acceptance tests pass against the live instance
- [ ] All v0.1 acceptance tests still pass (no regression)
- [ ] Live instance clean of `tf-acc-*` resources after the suite
- [ ] `slugify` removed from `application_resource.go`; only one definition exists in `database_helpers.go`
- [ ] All five resources registered in `internal/provider/provider.go`'s `Resources()` list
- [ ] `v0.2.0` tag pushed and GitHub Release published with 13 signed assets
- [ ] Registry shows `0.2.0`
