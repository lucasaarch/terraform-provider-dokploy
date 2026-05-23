# Dokploy Compose, Mounts, Ports, Notifications & App Advanced (v0.5) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `dokploy_compose`, `dokploy_mount`, `dokploy_port`, and five notification resources (Slack/Discord/Email/Telegram/Gotify); add `replicas`, `health_check`, and `restart_policy` to `dokploy_application`; add `compose` to the `database_type` enum of `dokploy_backup`. Ship as v0.5.0.

**Architecture:** Eight new resources follow the existing two-layer pattern. The five notification resources share one client file (`notification.go`) with type-specific `Create*` methods. `dokploy_compose` reuses `deployAndWait` from `database_helpers.go`. `dokploy_mount` uses a single resource with a `type` discriminator. `dokploy_application` gains nested single blocks for `health_check` and `restart_policy`.

**Tech Stack:** Go 1.26, `terraform-plugin-framework`, `terraform-plugin-framework-validators`. No new external dependencies.

**Spec:** `docs/superpowers/specs/2026-05-23-dokploy-compose-mounts-notifications-v0.5-design.md`

---

## Conventions for every task

- TDD: failing test first, see it fail, implement, see it pass, commit.
- Run `gofmt -w .` before every commit.
- Commit messages: conventional commits (`feat:`, `test:`, `chore:`, `docs:`).
- Unit tests use `httptest` and need no network. Acceptance tests (`TestAcc*`) require `TF_ACC=1` with `DOKPLOY_ENDPOINT` and `DOKPLOY_API_KEY` set (`source .dokploy-test-env`).
- End every commit message body with: `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>`.
- Acceptance tests hit the user's live Dokploy at `ship.sejablitz.com.br`. All test resource names use `tf-acc-` prefix. Confirm clean instance after the suite.
- `internal/client/API.md` is the source of truth — where this plan's code differs, API.md wins.

---

## Task 1: Verify compose/mount/port/notification routers + application advanced fields

Exploratory probe to fill all the v0.5 gaps before any code is written.

**Files:**
- Modify: `internal/client/API.md` (append sections)

- [ ] **Step 1: Load credentials**

```bash
cd /Users/lukearch/Projects/My/dokploy-terraform-provider
source .dokploy-test-env
```

- [ ] **Step 2: Probe compose router**

Create a throwaway project to probe against:

```bash
PROJ_RESP=$(/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d '{"name":"tf-probe-v05"}' "$DOKPLOY_ENDPOINT/api/project.create")
ENV_ID=$(echo "$PROJ_RESP" | /opt/homebrew/bin/python3 -c "import json,sys; print(json.load(sys.stdin)['environment']['environmentId'])")
echo "env: $ENV_ID"
```

Then exercise compose's endpoints:

```bash
# Get the required fields for a successful create.
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"name\":\"tf-probe-compose\",\"appName\":\"tf-probe-compose\",\"environmentId\":\"$ENV_ID\"}" \
  "$DOKPLOY_ENDPOINT/api/compose.create" | /opt/homebrew/bin/python3 -m json.tool

# Record: does compose.create return the full object or empty body? What fields?
```

Probe `compose.one`, `compose.update`, `compose.deploy`, `compose.remove` (try `.delete` if `.remove` fails). For each: HTTP method, path, required fields, response shape.

Record specifically:
- Does `compose.update` accept `sourceType`, `composeFile`, `env`? (We need it to save the YAML and env vars.)
- What does `compose.one` return — full object with embedded fields like `mounts[]`, `domains[]`?
- What is `applicationStatus` (or equivalent) on a compose service?

- [ ] **Step 3: Probe mount router**

```bash
# Get the per-type required fields by probing each type with the minimum.
APP_ID=<create a throwaway application; capture its id>

for TYPE in bind volume file; do
  echo "=== mount type=$TYPE ==="
  /usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
    -d "{\"type\":\"$TYPE\",\"mountPath\":\"/tmp\",\"serviceId\":\"$APP_ID\"}" \
    "$DOKPLOY_ENDPOINT/api/mounts.create" | /opt/homebrew/bin/python3 -m json.tool | head -30
done
```

Record per type:
- For `bind`: is `hostPath` the right field? Required?
- For `volume`: is `volumeName` the right field? Required?
- For `file`: is `content` the right field? Required?
- What does `mounts.one` return? Does it confirm `mountId` as the id field?
- What is the delete verb (`mounts.remove` or `mounts.delete`)?
- Is `serviceType` (application/compose/postgres/etc) required alongside `serviceId`?

- [ ] **Step 4: Probe port router**

```bash
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"applicationId\":\"$APP_ID\",\"publishedPort\":8080,\"targetPort\":80}" \
  "$DOKPLOY_ENDPOINT/api/port.create" | /opt/homebrew/bin/python3 -m json.tool | head -25

# Probe with protocol field included:
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"applicationId\":\"$APP_ID\",\"publishedPort\":8081,\"targetPort\":81,\"protocol\":\"tcp\"}" \
  "$DOKPLOY_ENDPOINT/api/port.create" | head -c 600
```

Record: does the API accept `protocol`? What are valid values? `port.one` response shape, update verb, delete verb.

- [ ] **Step 5: Probe notification.update and notification.remove/delete**

```bash
# First create a fake slack notification (it'll succeed because the URL isn't actually called)
SLACK_RESP=$(/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d '{"name":"tf-probe-slack","webhookUrl":"https://hooks.slack.com/services/T0/B0/X","channel":"#test","appBuildError":true,"databaseBackup":true,"dokployBackup":true,"volumeBackup":true,"dokployRestart":true,"appDeploy":true,"dockerCleanup":true,"serverThreshold":true}' \
  "$DOKPLOY_ENDPOINT/api/notification.createSlack")
echo "$SLACK_RESP" | /opt/homebrew/bin/python3 -m json.tool

NOTIF_ID=<extract from response>

# Try the universal update
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"notificationId\":\"$NOTIF_ID\",\"name\":\"renamed\"}" \
  "$DOKPLOY_ENDPOINT/api/notification.update" -w "\nHTTP %{http_code}\n"

# Try type-specific updates
for v in updateSlack updateDiscord updateEmail; do
  /usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
    -d "{\"notificationId\":\"$NOTIF_ID\"}" \
    "$DOKPLOY_ENDPOINT/api/notification.$v" -o /tmp/x -w "$v: HTTP %{http_code}\n"
done

# Try the remove + delete verbs
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"notificationId\":\"$NOTIF_ID\"}" "$DOKPLOY_ENDPOINT/api/notification.remove" -w "\nremove: HTTP %{http_code}\n"
```

Record: which update endpoints exist; which delete verb works.

- [ ] **Step 6: Probe notification.one to see if secrets are returned**

```bash
# Recreate the notification, then read it back
/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" \
  "$DOKPLOY_ENDPOINT/api/notification.one?notificationId=$NEW_NOTIF_ID" | /opt/homebrew/bin/python3 -m json.tool
```

Record: are `webhookUrl`/`botToken`/`password` returned in plaintext, or stripped? This drives the Read drift behavior.

- [ ] **Step 7: Probe application.update with advanced fields**

```bash
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"applicationId\":\"$APP_ID\",\"replicas\":3,\"healthCheckSwarm\":{\"Test\":[\"CMD\",\"echo\",\"hi\"],\"Interval\":30000000000,\"Timeout\":10000000000,\"Retries\":3,\"StartPeriod\":60000000000},\"restartPolicySwarm\":{\"Condition\":\"on-failure\",\"Delay\":5000000000,\"MaxAttempts\":3,\"Window\":120000000000}}" \
  "$DOKPLOY_ENDPOINT/api/application.update" -w "\nHTTP %{http_code}\n"

# Then fetch the application and inspect what shape healthCheckSwarm/restartPolicySwarm actually take
/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" "$DOKPLOY_ENDPOINT/api/application.one?applicationId=$APP_ID" \
  | /opt/homebrew/bin/python3 -c "
import json,sys
d = json.load(sys.stdin)
print('replicas:', d.get('replicas'))
print('healthCheckSwarm:', json.dumps(d.get('healthCheckSwarm'), indent=2))
print('restartPolicySwarm:', json.dumps(d.get('restartPolicySwarm'), indent=2))
"
```

Record: the exact JSON shape that `healthCheckSwarm` and `restartPolicySwarm` take in the API. (Docker Swarm uses nanosecond durations, but Dokploy may translate from string-style "30s"; verify.)

- [ ] **Step 8: Probe backup.create with databaseType=compose**

```bash
# Create a throwaway compose first (use the one from Step 2 if still alive).
COMPOSE_ID=<id from step 2>

/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"schedule\":\"0 3 * * *\",\"prefix\":\"tf-probe/\",\"destinationId\":\"FwQFgPCZe4wKraiAd_dyd\",\"database\":\"$COMPOSE_ID\",\"databaseType\":\"compose\",\"composeId\":\"$COMPOSE_ID\"}" \
  "$DOKPLOY_ENDPOINT/api/backup.create" -w "\nHTTP %{http_code}\n"

# Then fetch the compose and see if backups[] is now populated
/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" "$DOKPLOY_ENDPOINT/api/compose.one?composeId=$COMPOSE_ID" \
  | /opt/homebrew/bin/python3 -c "import json,sys; d=json.load(sys.stdin); print('backups:', json.dumps(d.get('backups',[]),indent=2)[:500])"
```

Record: does `databaseType=compose` work? Does `compose.one` return `backups[]`? This unlocks compose backup in v0.5.

- [ ] **Step 9: Investigate web-server (application) volume backups (v0.3 limitation)**

```bash
# Add a mount to the application first (the volume backups likely show up only after the app has at least one volume).
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"type\":\"volume\",\"mountPath\":\"/data\",\"volumeName\":\"tf-probe-data\",\"serviceId\":\"$APP_ID\"}" \
  "$DOKPLOY_ENDPOINT/api/mounts.create" | /opt/homebrew/bin/python3 -m json.tool

# Try the web-server backup
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"schedule\":\"0 3 * * *\",\"prefix\":\"tf-probe/\",\"destinationId\":\"FwQFgPCZe4wKraiAd_dyd\",\"database\":\"$APP_ID\",\"databaseType\":\"web-server\"}" \
  "$DOKPLOY_ENDPOINT/api/backup.create" -w "\nHTTP %{http_code}\n"

# Then look at application.one for backups[] or volumeBackups[]
/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" "$DOKPLOY_ENDPOINT/api/application.one?applicationId=$APP_ID" \
  | /opt/homebrew/bin/python3 -c "
import json,sys
d = json.load(sys.stdin)
print('backups field present:', 'backups' in d, 'len:', len(d.get('backups') or []))
print('volumeBackups field present:', 'volumeBackups' in d, 'len:', len(d.get('volumeBackups') or []))
print('all keys containing 'ackup':', [k for k in d.keys() if 'ackup' in k or 'Backup' in k])
"
```

Record: did the web-server backup persist? If yes, where do we list it? If we find the field, v0.5 can also resolve the v0.3 limitation (note in API.md and the relevant v0.5 task can extend `listBackupsForResource` in `backup.go`).

- [ ] **Step 10: Clean up every probe resource**

In reverse order: backups → mounts → ports → notifications → application → compose → project.

```bash
# Verify the live instance is clean.
/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" "$DOKPLOY_ENDPOINT/api/project.all" \
  | grep -oE '"name":"tf-probe[^"]*"'
# Should print nothing.
```

- [ ] **Step 11: Append five new sections to `internal/client/API.md`**

After the existing routers, add:

- `## compose.*` — methods, request bodies, response shape (including any `backups[]`/`mounts[]` lists), the deploy status field.
- `## mounts.*` — methods, per-type required fields, response shape of `mounts.one`, the delete verb.
- `## port.*` — methods, request body, response shape, what `protocol` values the API accepts.
- `## notification.*` — methods (the 5 type-specific creates, the update endpoint(s), the delete verb), response shape of `notification.one`, plaintext-secret note.
- A short addendum to the `application.*` section documenting the exact JSON shape of `healthCheckSwarm`, `restartPolicySwarm`, and the `replicas` field.
- A short addendum to the `backup.*` section if Step 8 confirmed `databaseType=compose` works, and/or if Step 9 found where web-server backups live.

- [ ] **Step 12: Commit**

```bash
git add internal/client/API.md
git commit -m "docs: API reference for compose, mounts, port, notification routers"
```

---

## Task 2: dokploy_compose resource

**Files:**
- Create: `internal/client/compose.go`
- Create: `internal/client/compose_test.go`
- Create: `internal/provider/compose_resource.go`
- Create: `internal/provider/compose_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewComposeResource`)

Adjust paths/verbs and the deploy-and-poll handling to match `API.md` from Task 1 — particularly if `compose.create` returns an empty body or returns the full object.

- [ ] **Step 1: Write the failing client unit tests**

`internal/client/compose_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateCompose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/compose.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body ComposeInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "monitoring" || body.EnvironmentID != "env1" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Compose{
			ID:            "co1",
			Name:          "monitoring",
			AppName:       "monitoring-abc",
			EnvironmentID: "env1",
			ComposeStatus: "idle",
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	co, err := c.CreateCompose(context.Background(), ComposeInput{
		Name:          "monitoring",
		AppName:       "monitoring",
		EnvironmentID: "env1",
	})
	if err != nil {
		t.Fatalf("CreateCompose() error = %v", err)
	}
	if co.ID != "co1" {
		t.Errorf("co = %+v", co)
	}
}

func TestGetCompose_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetCompose(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Compose -v` — Expected: FAIL — `undefined: Compose`.

- [ ] **Step 3: Write `internal/client/compose.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Compose is a Docker Compose stack managed by Dokploy.
// The deploy-status field is named composeStatus (NOT applicationStatus).
type Compose struct {
	ID            string   `json:"composeId"`
	Name          string   `json:"name"`
	AppName       string   `json:"appName"`
	Description   string   `json:"description"`
	EnvironmentID string   `json:"environmentId"`
	ServerID      *string  `json:"serverId"`
	SourceType    string   `json:"sourceType"`
	ComposeFile   string   `json:"composeFile"`
	Env           string   `json:"env"`
	ComposeStatus string   `json:"composeStatus"`
	Backups       []Backup `json:"backups"`
}

// ComposeInput is the create/update payload.
type ComposeInput struct {
	Name          string  `json:"name,omitempty"`
	AppName       string  `json:"appName,omitempty"`
	Description   string  `json:"description,omitempty"`
	EnvironmentID string  `json:"environmentId,omitempty"`
	ServerID      *string `json:"serverId,omitempty"`
	SourceType    string  `json:"sourceType,omitempty"`
	ComposeFile   string  `json:"composeFile,omitempty"`
	Env           string  `json:"env,omitempty"`
}

func (c *Client) CreateCompose(ctx context.Context, in ComposeInput) (*Compose, error) {
	var out Compose
	if err := c.do(ctx, http.MethodPost, "compose.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetCompose(ctx context.Context, id string) (*Compose, error) {
	var out Compose
	q := url.Values{"composeId": {id}}
	if err := c.do(ctx, http.MethodGet, "compose.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateCompose(ctx context.Context, id string, in ComposeInput) error {
	payload := struct {
		ComposeInput
		ID string `json:"composeId"`
	}{ComposeInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "compose.update", payload, nil, nil)
}

// DeleteCompose calls compose.delete (the API uses .delete, not .remove, for
// this router — verified in Task 1's API.md).
func (c *Client) DeleteCompose(ctx context.Context, id string) error {
	payload := map[string]string{"composeId": id}
	return c.do(ctx, http.MethodPost, "compose.delete", payload, nil, nil)
}

// DeployCompose triggers an asynchronous deployment of the stack.
func (c *Client) DeployCompose(ctx context.Context, id string) error {
	payload := map[string]string{"composeId": id}
	return c.do(ctx, http.MethodPost, "compose.deploy", payload, nil, nil)
}
```

> If Task 1 found that `compose.create` returns an empty body (like sshKey/backup), wrap `CreateCompose` in a list-then-diff pattern: list `compose.all` (or via `env.one`/`project.one`'s compose list) before, create, list after, find the new id. The acceptance test will catch a wrong assumption immediately.

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Compose -v` — Expected: PASS.

- [ ] **Step 5: Write the failing acceptance test**

`internal/provider/compose_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccComposeResource(t *testing.T) {
	suffix := randInt()
	composeYAML := `version: "3"
services:
  hello:
    image: nginx:alpine
    restart: unless-stopped
`
	config := func(env string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-compose-proj-%d"
}

resource "dokploy_compose" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-compose"
  compose_file   = %q
  env = {
    HELLO = %q
  }
  timeouts {
    create = "15m"
    update = "15m"
  }
}`, suffix, composeYAML, env)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_compose.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_compose.test", "app_name"),
					resource.TestCheckResourceAttr("dokploy_compose.test", "status", "done"),
				),
			},
			{
				ResourceName:            "dokploy_compose.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				Config: config("v2"),
				Check:  resource.TestCheckResourceAttr("dokploy_compose.test", "env.HELLO", "v2"),
			},
		},
	})
}
```

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewComposeResource`.

- [ ] **Step 7: Write `internal/provider/compose_resource.go`**

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &composeResource{}
	_ resource.ResourceWithConfigure   = &composeResource{}
	_ resource.ResourceWithImportState = &composeResource{}
)

type composeResource struct{ client *client.Client }

func NewComposeResource() resource.Resource { return &composeResource{} }

type composeModel struct {
	ID            types.String   `tfsdk:"id"`
	EnvironmentID types.String   `tfsdk:"environment_id"`
	Name          types.String   `tfsdk:"name"`
	Description   types.String   `tfsdk:"description"`
	ComposeFile   types.String   `tfsdk:"compose_file"`
	SourceType    types.String   `tfsdk:"source_type"`
	Env           types.Map      `tfsdk:"env"`
	ServerID      types.String   `tfsdk:"server_id"`
	AppName       types.String   `tfsdk:"app_name"`
	Status        types.String   `tfsdk:"status"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func (r *composeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compose"
}

func (r *composeResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Docker Compose stack managed by Dokploy. v0.5 supports source_type \"raw\" only (paste the YAML inline).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"environment_id": schema.StringAttribute{Required: true, MarkdownDescription: "Environment that owns the stack. Changing forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":         schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description":  schema.StringAttribute{Optional: true, MarkdownDescription: "Description. Once set, removing this attribute does not clear it on the server."},
			"compose_file": schema.StringAttribute{Required: true, MarkdownDescription: "Contents of the docker-compose.yml file."},
			"source_type":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("raw"), MarkdownDescription: "Source type. v0.5 only supports `raw` (inline YAML)."},
			"env":          schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Env vars passed to the stack."},
			"server_id":    schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Managed server ID. Omit to run on the Dokploy host. Changing forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}},
			"app_name":     schema.StringAttribute{Computed: true, MarkdownDescription: "Internal name generated by Dokploy.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"status":       schema.StringAttribute{Computed: true, MarkdownDescription: "Status of the most recent deploy."},
			"timeouts":     timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *composeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *composeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan composeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, diags := plan.Timeouts.Create(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}

	co, err := r.client.CreateCompose(ctx, client.ComposeInput{
		Name:          plan.Name.ValueString(),
		AppName:       slugify(plan.Name.ValueString()),
		Description:   plan.Description.ValueString(),
		EnvironmentID: plan.EnvironmentID.ValueString(),
		ServerID:      optionalString(plan.ServerID),
		SourceType:    plan.SourceType.ValueString(),
		ComposeFile:   plan.ComposeFile.ValueString(),
		Env:           envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating compose", err.Error())
		return
	}
	plan.ID = types.StringValue(co.ID)
	plan.AppName = types.StringValue(co.AppName)

	// Apply compose_file / env via update (some APIs require save endpoints).
	if err := r.client.UpdateCompose(ctx, co.ID, client.ComposeInput{
		SourceType:  plan.SourceType.ValueString(),
		ComposeFile: plan.ComposeFile.ValueString(),
		Env:         envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error saving compose configuration", err.Error())
		return
	}

	deployFn := func(ctx context.Context) error { return r.client.DeployCompose(ctx, co.ID) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetCompose(ctx, co.ID)
		if err != nil {
			return "", err
		}
		return got.ComposeStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Compose deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *composeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state composeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	co, err := r.client.GetCompose(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading compose", err.Error())
		return
	}
	state.Name = types.StringValue(co.Name)
	state.EnvironmentID = types.StringValue(co.EnvironmentID)
	state.AppName = types.StringValue(co.AppName)
	state.Status = types.StringValue(co.ComposeStatus)
	state.ComposeFile = types.StringValue(co.ComposeFile)
	state.SourceType = types.StringValue(co.SourceType)
	if co.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(co.Description)
	}
	if co.ServerID != nil {
		state.ServerID = types.StringValue(*co.ServerID)
	} else {
		state.ServerID = types.StringNull()
	}
	if co.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(co.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *composeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan composeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	if err := r.client.UpdateCompose(ctx, plan.ID.ValueString(), client.ComposeInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		SourceType:  plan.SourceType.ValueString(),
		ComposeFile: plan.ComposeFile.ValueString(),
		Env:         envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating compose", err.Error())
		return
	}

	deployFn := func(ctx context.Context) error { return r.client.DeployCompose(ctx, plan.ID.ValueString()) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetCompose(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ComposeStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Compose deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *composeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state composeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteCompose(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting compose", err.Error())
	}
}

func (r *composeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run acceptance test**

Append `NewComposeResource,` to `internal/provider/provider.go::Resources()`.

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run TestAccComposeResource -v -timeout 30m
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/client/compose.go internal/client/compose_test.go \
        internal/provider/compose_resource.go internal/provider/compose_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: dokploy_compose resource"
```

---

## Task 3: dokploy_mount resource (with bind/volume/file discriminator)

**Files:**
- Create: `internal/client/mount.go`
- Create: `internal/client/mount_test.go`
- Create: `internal/provider/mount_resource.go`
- Create: `internal/provider/mount_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewMountResource`)

- [ ] **Step 1: Write the failing client unit tests**

`internal/client/mount_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateMount_Bind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mounts.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body MountInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Type != "bind" || body.HostPath != "/srv/data" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Mount{ID: "m1", Type: "bind", MountPath: body.MountPath, HostPath: body.HostPath})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	m, err := c.CreateMount(context.Background(), MountInput{
		ServiceID: "app1",
		Type:      "bind",
		MountPath: "/data",
		HostPath:  "/srv/data",
	})
	if err != nil {
		t.Fatalf("CreateMount() error = %v", err)
	}
	if m.ID != "m1" {
		t.Errorf("m = %+v", m)
	}
}

func TestCreateMount_Volume(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body MountInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Type != "volume" || body.VolumeName != "datavol" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Mount{ID: "m2", Type: "volume", VolumeName: body.VolumeName})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	m, err := c.CreateMount(context.Background(), MountInput{
		ServiceID:  "app1",
		Type:       "volume",
		MountPath:  "/data",
		VolumeName: "datavol",
	})
	if err != nil {
		t.Fatalf("CreateMount() error = %v", err)
	}
	if m.ID != "m2" {
		t.Errorf("m = %+v", m)
	}
}

func TestCreateMount_File(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body MountInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Type != "file" || body.Content != "hello\n" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Mount{ID: "m3", Type: "file"})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	m, err := c.CreateMount(context.Background(), MountInput{
		ServiceID: "app1",
		Type:      "file",
		MountPath: "/etc/config.yml",
		Content:   "hello\n",
	})
	if err != nil {
		t.Fatalf("CreateMount() error = %v", err)
	}
	if m.ID != "m3" {
		t.Errorf("m = %+v", m)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Mount -v` — Expected: FAIL.

- [ ] **Step 3: Write `internal/client/mount.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Mount is a volume/bind/file mount on a Dokploy service.
// `serviceId` is write-only on create; the API does not return it. On read,
// the parent is identified via `serviceType` + a nullable per-type id field.
type Mount struct {
	ID          string  `json:"mountId"`
	Type        string  `json:"type"`
	MountPath   string  `json:"mountPath"`
	HostPath    string  `json:"hostPath"`
	VolumeName  string  `json:"volumeName"`
	Content     string  `json:"content"`
	ServiceType string  `json:"serviceType"`
	// Exactly one of these will be non-null on a read response.
	ApplicationID *string `json:"applicationId"`
	ComposeID     *string `json:"composeId"`
	PostgresID    *string `json:"postgresId"`
	MysqlID       *string `json:"mysqlId"`
	MariadbID     *string `json:"mariadbId"`
	MongoID       *string `json:"mongoId"`
	RedisID       *string `json:"redisId"`
}

// ResolveServiceID returns the parent service id by inspecting the nullable
// per-type id fields populated by the API on read.
func (m *Mount) ResolveServiceID() string {
	for _, p := range []*string{m.ApplicationID, m.ComposeID, m.PostgresID, m.MysqlID, m.MariadbID, m.MongoID, m.RedisID} {
		if p != nil && *p != "" {
			return *p
		}
	}
	return ""
}

// MountInput is the create/update payload. Per-type required fields:
//   bind   -> HostPath
//   volume -> VolumeName
//   file   -> Content
type MountInput struct {
	ServiceID  string `json:"serviceId,omitempty"`
	Type       string `json:"type,omitempty"`
	MountPath  string `json:"mountPath,omitempty"`
	HostPath   string `json:"hostPath,omitempty"`
	VolumeName string `json:"volumeName,omitempty"`
	Content    string `json:"content,omitempty"`
}

func (c *Client) CreateMount(ctx context.Context, in MountInput) (*Mount, error) {
	var out Mount
	if err := c.do(ctx, http.MethodPost, "mounts.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMount(ctx context.Context, id string) (*Mount, error) {
	var out Mount
	q := url.Values{"mountId": {id}}
	if err := c.do(ctx, http.MethodGet, "mounts.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMount(ctx context.Context, id string, in MountInput) error {
	payload := struct {
		MountInput
		ID string `json:"mountId"`
	}{MountInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "mounts.update", payload, nil, nil)
}

func (c *Client) DeleteMount(ctx context.Context, id string) error {
	payload := map[string]string{"mountId": id}
	return c.do(ctx, http.MethodPost, "mounts.remove", payload, nil, nil)
}
```

> If Task 1 found different delete/update verbs, adjust.

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Mount -v` — Expected: PASS.

- [ ] **Step 5: Write the failing acceptance test**

`internal/provider/mount_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMountResource_Bind(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-mount-bind-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-mount-bind-app"
  docker_image   = "nginx:alpine"
  timeouts { create = "15m" update = "15m" }
}

resource "dokploy_mount" "test" {
  service_id = dokploy_application.test.id
  type       = "bind"
  mount_path = "/srv/static"
  host_path  = "/var/www/static"
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mount.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mount.test", "type", "bind"),
					resource.TestCheckResourceAttr("dokploy_mount.test", "host_path", "/var/www/static"),
				),
			},
			{
				ResourceName:      "dokploy_mount.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccMountResource_Volume(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-mount-vol-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-mount-vol-app"
  docker_image   = "nginx:alpine"
  timeouts { create = "15m" update = "15m" }
}

resource "dokploy_mount" "test" {
  service_id  = dokploy_application.test.id
  type        = "volume"
  mount_path  = "/data"
  volume_name = "tf-acc-volume-%d"
}`, suffix, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mount.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mount.test", "type", "volume"),
				),
			},
		},
	})
}

func TestAccMountResource_File(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-mount-file-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-mount-file-app"
  docker_image   = "nginx:alpine"
  timeouts { create = "15m" update = "15m" }
}

resource "dokploy_mount" "test" {
  service_id = dokploy_application.test.id
  type       = "file"
  mount_path = "/etc/nginx/conf.d/extra.conf"
  content    = "client_max_body_size 100M;\n"
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mount.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mount.test", "type", "file"),
				),
			},
		},
	})
}
```

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewMountResource`.

- [ ] **Step 7: Write `internal/provider/mount_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &mountResource{}
	_ resource.ResourceWithConfigure   = &mountResource{}
	_ resource.ResourceWithImportState = &mountResource{}
)

type mountResource struct{ client *client.Client }

func NewMountResource() resource.Resource { return &mountResource{} }

type mountModel struct {
	ID         types.String `tfsdk:"id"`
	ServiceID  types.String `tfsdk:"service_id"`
	Type       types.String `tfsdk:"type"`
	MountPath  types.String `tfsdk:"mount_path"`
	HostPath   types.String `tfsdk:"host_path"`
	VolumeName types.String `tfsdk:"volume_name"`
	Content    types.String `tfsdk:"content"`
}

func (r *mountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mount"
}

func (r *mountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A mount (bind, volume, or file) attached to a Dokploy service (application, compose, postgres, etc).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"service_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the service that owns the mount (`dokploy_application.x.id`, `dokploy_compose.x.id`, `dokploy_postgres.x.id`, etc). Changing forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Mount type. One of `bind`, `volume`, `file`.",
				Validators:          []validator.String{stringvalidator.OneOf("bind", "volume", "file")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"mount_path": schema.StringAttribute{Required: true, MarkdownDescription: "Path inside the container."},
			"host_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Host path. Required when `type = \"bind\"`.",
			},
			"volume_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Docker volume name. Required when `type = \"volume\"`.",
			},
			"content": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "File contents. Required when `type = \"file\"`.",
			},
		},
	}
}

func (r *mountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ValidateConfig enforces the per-type required-field rules at plan time.
func (r *mountResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg mountModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	switch cfg.Type.ValueString() {
	case "bind":
		if cfg.HostPath.IsNull() || cfg.HostPath.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(path.Root("host_path"), "host_path is required when type = \"bind\"", "")
		}
		if !cfg.VolumeName.IsNull() || !cfg.Content.IsNull() {
			resp.Diagnostics.AddError("Conflicting attributes", "When type = \"bind\", do not set volume_name or content.")
		}
	case "volume":
		if cfg.VolumeName.IsNull() || cfg.VolumeName.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(path.Root("volume_name"), "volume_name is required when type = \"volume\"", "")
		}
		if !cfg.HostPath.IsNull() || !cfg.Content.IsNull() {
			resp.Diagnostics.AddError("Conflicting attributes", "When type = \"volume\", do not set host_path or content.")
		}
	case "file":
		if cfg.Content.IsNull() || cfg.Content.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(path.Root("content"), "content is required when type = \"file\"", "")
		}
		if !cfg.HostPath.IsNull() || !cfg.VolumeName.IsNull() {
			resp.Diagnostics.AddError("Conflicting attributes", "When type = \"file\", do not set host_path or volume_name.")
		}
	}
}

func (m mountModel) toInput() client.MountInput {
	return client.MountInput{
		ServiceID:  m.ServiceID.ValueString(),
		Type:       m.Type.ValueString(),
		MountPath:  m.MountPath.ValueString(),
		HostPath:   m.HostPath.ValueString(),
		VolumeName: m.VolumeName.ValueString(),
		Content:    m.Content.ValueString(),
	}
}

func (r *mountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := r.client.CreateMount(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating mount", err.Error())
		return
	}
	plan.ID = types.StringValue(m.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := r.client.GetMount(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading mount", err.Error())
		return
	}
	if resolved := m.ResolveServiceID(); resolved != "" {
		state.ServiceID = types.StringValue(resolved)
	}
	state.Type = types.StringValue(m.Type)
	state.MountPath = types.StringValue(m.MountPath)
	if m.HostPath != "" {
		state.HostPath = types.StringValue(m.HostPath)
	}
	if m.VolumeName != "" {
		state.VolumeName = types.StringValue(m.VolumeName)
	}
	if m.Content != "" {
		state.Content = types.StringValue(m.Content)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *mountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdateMount(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating mount", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMount(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting mount", err.Error())
	}
}

func (r *mountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run all three acceptance tests**

Append `NewMountResource,` to `Resources()`.

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run "TestAccMountResource_" -v -timeout 30m
```

Expected: all three (Bind/Volume/File) PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/client/mount.go internal/client/mount_test.go \
        internal/provider/mount_resource.go internal/provider/mount_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: dokploy_mount resource (bind/volume/file)"
```

---

## Task 4: dokploy_port resource

**Files:**
- Create: `internal/client/port.go`
- Create: `internal/client/port_test.go`
- Create: `internal/provider/port_resource.go`
- Create: `internal/provider/port_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewPortResource`)

- [ ] **Step 1: Write failing client unit tests**

`internal/client/port_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreatePort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/port.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body PortInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.PublishedPort != 8080 || body.TargetPort != 80 {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Port{
			ID:            "p1",
			ApplicationID: body.ApplicationID,
			PublishedPort: body.PublishedPort,
			TargetPort:    body.TargetPort,
			Protocol:      "tcp",
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	p, err := c.CreatePort(context.Background(), PortInput{
		ApplicationID: "app1",
		PublishedPort: 8080,
		TargetPort:    80,
		Protocol:      "tcp",
	})
	if err != nil {
		t.Fatalf("CreatePort() error = %v", err)
	}
	if p.ID != "p1" {
		t.Errorf("p = %+v", p)
	}
}

func TestGetPort_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetPort(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Port -v` — Expected: FAIL.

- [ ] **Step 3: Write `internal/client/port.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Port is a published port mapping on an application.
type Port struct {
	ID            string `json:"portId"`
	ApplicationID string `json:"applicationId"`
	PublishedPort int    `json:"publishedPort"`
	TargetPort    int    `json:"targetPort"`
	Protocol      string `json:"protocol"`
}

// PortInput is the create/update payload.
type PortInput struct {
	ApplicationID string `json:"applicationId,omitempty"`
	PublishedPort int    `json:"publishedPort,omitempty"`
	TargetPort    int    `json:"targetPort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

func (c *Client) CreatePort(ctx context.Context, in PortInput) (*Port, error) {
	var out Port
	if err := c.do(ctx, http.MethodPost, "port.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPort(ctx context.Context, id string) (*Port, error) {
	var out Port
	q := url.Values{"portId": {id}}
	if err := c.do(ctx, http.MethodGet, "port.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdatePort(ctx context.Context, id string, in PortInput) error {
	payload := struct {
		PortInput
		ID string `json:"portId"`
	}{PortInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "port.update", payload, nil, nil)
}

func (c *Client) DeletePort(ctx context.Context, id string) error {
	payload := map[string]string{"portId": id}
	return c.do(ctx, http.MethodPost, "port.remove", payload, nil, nil)
}
```

> If Task 1 found a different delete verb or that `protocol` is rejected, adjust.

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Port -v` — Expected: PASS.

- [ ] **Step 5: Write the failing acceptance test**

`internal/provider/port_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPortResource(t *testing.T) {
	suffix := randInt()
	config := func(target int) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-port-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-port-app"
  docker_image   = "nginx:alpine"
  timeouts { create = "15m" update = "15m" }
}

resource "dokploy_port" "test" {
  application_id  = dokploy_application.test.id
  published_port  = 8080
  target_port     = %d
}`, suffix, target)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(80),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_port.test", "id"),
					resource.TestCheckResourceAttr("dokploy_port.test", "published_port", "8080"),
					resource.TestCheckResourceAttr("dokploy_port.test", "target_port", "80"),
				),
			},
			{
				ResourceName:      "dokploy_port.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config(8080),
				Check:  resource.TestCheckResourceAttr("dokploy_port.test", "target_port", "8080"),
			},
		},
	})
}
```

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewPortResource`.

- [ ] **Step 7: Write `internal/provider/port_resource.go`**

```go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &portResource{}
	_ resource.ResourceWithConfigure   = &portResource{}
	_ resource.ResourceWithImportState = &portResource{}
)

type portResource struct{ client *client.Client }

func NewPortResource() resource.Resource { return &portResource{} }

type portModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	PublishedPort types.Int64  `tfsdk:"published_port"`
	TargetPort    types.Int64  `tfsdk:"target_port"`
	Protocol      types.String `tfsdk:"protocol"`
}

func (r *portResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port"
}

func (r *portResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A published port on a Dokploy application (host port → container port).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"application_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application that owns the port. Changing forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"published_port": schema.Int64Attribute{Required: true, MarkdownDescription: "Host port (published)."},
			"target_port":    schema.Int64Attribute{Required: true, MarkdownDescription: "Container port (target)."},
			"protocol": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("tcp"),
				MarkdownDescription: "`tcp` (default) or `udp`.",
			},
		},
	}
}

func (r *portResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m portModel) toInput() client.PortInput {
	return client.PortInput{
		ApplicationID: m.ApplicationID.ValueString(),
		PublishedPort: int(m.PublishedPort.ValueInt64()),
		TargetPort:    int(m.TargetPort.ValueInt64()),
		Protocol:      m.Protocol.ValueString(),
	}
}

func (r *portResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan portModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.CreatePort(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating port", err.Error())
		return
	}
	plan.ID = types.StringValue(p.ID)
	if p.Protocol != "" {
		plan.Protocol = types.StringValue(p.Protocol)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *portResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state portModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.GetPort(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading port", err.Error())
		return
	}
	state.ApplicationID = types.StringValue(p.ApplicationID)
	state.PublishedPort = types.Int64Value(int64(p.PublishedPort))
	state.TargetPort = types.Int64Value(int64(p.TargetPort))
	state.Protocol = types.StringValue(p.Protocol)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *portResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan portModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdatePort(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating port", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *portResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state portModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePort(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting port", err.Error())
	}
}

func (r *portResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run acceptance test**

Append `NewPortResource,` to `Resources()`.

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run TestAccPortResource -v -timeout 30m
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/client/port.go internal/client/port_test.go \
        internal/provider/port_resource.go internal/provider/port_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: dokploy_port resource"
```

---

## Task 5: Notification client + five notification resources

> **Critical Task 1 findings — apply these corrections to the plan code below:**
>
> 1. **All 5 `notification.create<Type>` endpoints return HTTP 200 with empty body.** The new id must be discovered via `notification.all` diff (same pattern as `sshKey.create` in v0.4). Implement `ListNotifications` and wrap every `Create<Type>Notification` with a list-before / call / list-after / find-new-id flow.
> 2. **There is NO universal `notification.update`.** Updates are 5 type-specific endpoints: `notification.updateSlack`, `notification.updateDiscord`, `notification.updateEmail`, `notification.updateTelegram`, `notification.updateGotify`. Each one requires BOTH `notificationId` AND the type-specific sub-id (`slackId`/`discordId`/`emailId`/`telegramId`/`gotifyId`) read from `notification.one`.
> 3. **`notification.one` returns these type-specific sub-ids.** Add `SlackID`, `DiscordID`, `EmailID`, `TelegramID`, `GotifyID` (each as `*string`) to the `Notification` struct.
> 4. **Each Terraform resource must expose its sub-id as a Computed attribute** (e.g. `slack_id` on `dokploy_slack_notification`) so the Update method can read it from state and pass to the type-specific update endpoint.
> 5. The client method signature is therefore `UpdateSlackNotification(ctx, notificationId, slackId string, in SlackNotificationInput) error` (and same shape for the other four). Updates also return empty body.
> 6. **`notification.one` returns webhook URLs / bot tokens / SMTP passwords in plaintext** — Read can overwrite state values directly (drift detection works for sensitive values).
>
> The plan code below DOES NOT yet reflect these corrections. The implementer must apply them while writing the code — using the v0.4 sshKey diff-pattern and v0.2 password-handling patterns as references.

This task adds the shared notification client and all five type-specific resources. The Slack resource is shown in full; the other four follow the same shape with type-specific fields documented in tables.

**Files:**
- Create: `internal/client/notification.go`
- Create: `internal/client/notification_test.go`
- Create: `internal/provider/slack_notification_resource.go`
- Create: `internal/provider/slack_notification_resource_test.go`
- Create: `internal/provider/discord_notification_resource.go`
- Create: `internal/provider/discord_notification_resource_test.go`
- Create: `internal/provider/email_notification_resource.go`
- Create: `internal/provider/email_notification_resource_test.go`
- Create: `internal/provider/telegram_notification_resource.go`
- Create: `internal/provider/telegram_notification_resource_test.go`
- Create: `internal/provider/gotify_notification_resource.go`
- Create: `internal/provider/gotify_notification_resource_test.go`
- Modify: `internal/provider/provider.go` (register all 5)

- [ ] **Step 1: Write failing client unit tests**

`internal/client/notification_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSlackNotification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/notification.createSlack" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body SlackNotificationInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.WebhookURL == "" || body.Channel == "" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(Notification{ID: "n1", Name: body.Name, NotificationType: "slack"})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	n, err := c.CreateSlackNotification(context.Background(), SlackNotificationInput{
		Name:            "alerts",
		WebhookURL:      "https://hooks.slack.com/services/T0/B0/X",
		Channel:         "#deploys",
		AppDeploy:       true,
		AppBuildError:   true,
		DatabaseBackup:  true,
		DokployBackup:   true,
		VolumeBackup:    true,
		DokployRestart:  true,
		DockerCleanup:   true,
		ServerThreshold: true,
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Notification -v` — Expected: FAIL.

- [ ] **Step 3: Write `internal/client/notification.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// EventFlags are the eight event toggles common to every notification type.
type EventFlags struct {
	AppDeploy       bool `json:"appDeploy"`
	AppBuildError   bool `json:"appBuildError"`
	DatabaseBackup  bool `json:"databaseBackup"`
	DokployBackup   bool `json:"dokployBackup"`
	VolumeBackup    bool `json:"volumeBackup"`
	DokployRestart  bool `json:"dokployRestart"`
	DockerCleanup   bool `json:"dockerCleanup"`
	ServerThreshold bool `json:"serverThreshold"`
}

// Notification is the read shape returned by notification.one. The presence of
// type-specific fields varies based on NotificationType.
type Notification struct {
	ID               string `json:"notificationId"`
	Name             string `json:"name"`
	NotificationType string `json:"notificationType"`
	EventFlags
	// Slack/Discord
	WebhookURL string `json:"webhookUrl"`
	Channel    string `json:"channel"`
	Decoration *bool  `json:"decoration"`
	// Email
	SMTPServer   string   `json:"smtpServer"`
	SMTPPort     int      `json:"smtpPort"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	FromAddress  string   `json:"fromAddress"`
	ToAddresses  []string `json:"toAddresses"`
	// Telegram
	BotToken        string `json:"botToken"`
	ChatID          string `json:"chatId"`
	MessageThreadID string `json:"messageThreadId"`
	// Gotify
	ServerURL string `json:"serverUrl"`
	AppToken  string `json:"appToken"`
	Priority  *int   `json:"priority"`
}

// SlackNotificationInput is the payload for notification.createSlack.
type SlackNotificationInput struct {
	Name       string `json:"name,omitempty"`
	WebhookURL string `json:"webhookUrl,omitempty"`
	Channel    string `json:"channel,omitempty"`
	EventFlags
}

// DiscordNotificationInput is the payload for notification.createDiscord.
type DiscordNotificationInput struct {
	Name       string `json:"name,omitempty"`
	WebhookURL string `json:"webhookUrl,omitempty"`
	Decoration *bool  `json:"decoration,omitempty"`
	EventFlags
}

// EmailNotificationInput is the payload for notification.createEmail.
type EmailNotificationInput struct {
	Name        string   `json:"name,omitempty"`
	SMTPServer  string   `json:"smtpServer,omitempty"`
	SMTPPort    int      `json:"smtpPort,omitempty"`
	Username    string   `json:"username,omitempty"`
	Password    string   `json:"password,omitempty"`
	FromAddress string   `json:"fromAddress,omitempty"`
	ToAddresses []string `json:"toAddresses,omitempty"`
	EventFlags
}

// TelegramNotificationInput is the payload for notification.createTelegram.
type TelegramNotificationInput struct {
	Name            string `json:"name,omitempty"`
	BotToken        string `json:"botToken,omitempty"`
	ChatID          string `json:"chatId,omitempty"`
	MessageThreadID string `json:"messageThreadId,omitempty"`
	EventFlags
}

// GotifyNotificationInput is the payload for notification.createGotify.
type GotifyNotificationInput struct {
	Name       string `json:"name,omitempty"`
	ServerURL  string `json:"serverUrl,omitempty"`
	AppToken   string `json:"appToken,omitempty"`
	Priority   *int   `json:"priority,omitempty"`
	Decoration *bool  `json:"decoration,omitempty"`
	EventFlags
}

func (c *Client) CreateSlackNotification(ctx context.Context, in SlackNotificationInput) (*Notification, error) {
	var out Notification
	if err := c.do(ctx, http.MethodPost, "notification.createSlack", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateDiscordNotification(ctx context.Context, in DiscordNotificationInput) (*Notification, error) {
	var out Notification
	if err := c.do(ctx, http.MethodPost, "notification.createDiscord", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateEmailNotification(ctx context.Context, in EmailNotificationInput) (*Notification, error) {
	var out Notification
	if err := c.do(ctx, http.MethodPost, "notification.createEmail", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateTelegramNotification(ctx context.Context, in TelegramNotificationInput) (*Notification, error) {
	var out Notification
	if err := c.do(ctx, http.MethodPost, "notification.createTelegram", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateGotifyNotification(ctx context.Context, in GotifyNotificationInput) (*Notification, error) {
	var out Notification
	if err := c.do(ctx, http.MethodPost, "notification.createGotify", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetNotification(ctx context.Context, id string) (*Notification, error) {
	var out Notification
	q := url.Values{"notificationId": {id}}
	if err := c.do(ctx, http.MethodGet, "notification.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteNotification(ctx context.Context, id string) error {
	payload := map[string]string{"notificationId": id}
	return c.do(ctx, http.MethodPost, "notification.remove", payload, nil, nil)
}
```

> Task 1 should have confirmed the exact `notification.update` endpoint(s). If updates are universal (`notification.update`), add a single `UpdateNotification` method that takes the same shape as the create input plus `notificationId`. If updates are type-specific (`updateSlack`/`updateDiscord`/etc), add `Update<Type>Notification` methods. Pick the shape that matches the API and add the methods to this file.

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Notification -v` — Expected: PASS.

- [ ] **Step 5: Write `internal/provider/slack_notification_resource.go`**

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &slackNotificationResource{}
	_ resource.ResourceWithConfigure   = &slackNotificationResource{}
	_ resource.ResourceWithImportState = &slackNotificationResource{}
)

type slackNotificationResource struct{ client *client.Client }

func NewSlackNotificationResource() resource.Resource { return &slackNotificationResource{} }

type slackNotificationModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	WebhookURL      types.String `tfsdk:"webhook_url"`
	Channel         types.String `tfsdk:"channel"`
	AppDeploy       types.Bool   `tfsdk:"app_deploy"`
	AppBuildError   types.Bool   `tfsdk:"app_build_error"`
	DatabaseBackup  types.Bool   `tfsdk:"database_backup"`
	DokployBackup   types.Bool   `tfsdk:"dokploy_backup"`
	VolumeBackup    types.Bool   `tfsdk:"volume_backup"`
	DokployRestart  types.Bool   `tfsdk:"dokploy_restart"`
	DockerCleanup   types.Bool   `tfsdk:"docker_cleanup"`
	ServerThreshold types.Bool   `tfsdk:"server_threshold"`
}

func (r *slackNotificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_slack_notification"
}

func (r *slackNotificationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Slack notification configuration. Dokploy posts deploy/backup/restart events to the configured webhook.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"webhook_url": schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "Slack incoming webhook URL."},
			"channel":     schema.StringAttribute{Required: true, MarkdownDescription: "Slack channel (e.g. `#deploys`)."},
			"app_deploy":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on application deploy."},
			"app_build_error":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on build error."},
			"database_backup":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on database backup events."},
			"dokploy_backup":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on Dokploy self-backup events."},
			"volume_backup":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on volume backup events."},
			"dokploy_restart":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on Dokploy restart."},
			"docker_cleanup":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on Docker cleanup."},
			"server_threshold": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on server resource threshold breaches."},
		},
	}
}

func (r *slackNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m slackNotificationModel) toInput() client.SlackNotificationInput {
	return client.SlackNotificationInput{
		Name:       m.Name.ValueString(),
		WebhookURL: m.WebhookURL.ValueString(),
		Channel:    m.Channel.ValueString(),
		EventFlags: client.EventFlags{
			AppDeploy:       m.AppDeploy.ValueBool(),
			AppBuildError:   m.AppBuildError.ValueBool(),
			DatabaseBackup:  m.DatabaseBackup.ValueBool(),
			DokployBackup:   m.DokployBackup.ValueBool(),
			VolumeBackup:    m.VolumeBackup.ValueBool(),
			DokployRestart:  m.DokployRestart.ValueBool(),
			DockerCleanup:   m.DockerCleanup.ValueBool(),
			ServerThreshold: m.ServerThreshold.ValueBool(),
		},
	}
}

func (r *slackNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan slackNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n, err := r.client.CreateSlackNotification(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating slack notification", err.Error())
		return
	}
	plan.ID = types.StringValue(n.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *slackNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state slackNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n, err := r.client.GetNotification(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading slack notification", err.Error())
		return
	}
	state.Name = types.StringValue(n.Name)
	if n.WebhookURL != "" {
		state.WebhookURL = types.StringValue(n.WebhookURL)
	}
	state.Channel = types.StringValue(n.Channel)
	state.AppDeploy = types.BoolValue(n.AppDeploy)
	state.AppBuildError = types.BoolValue(n.AppBuildError)
	state.DatabaseBackup = types.BoolValue(n.DatabaseBackup)
	state.DokployBackup = types.BoolValue(n.DokployBackup)
	state.VolumeBackup = types.BoolValue(n.VolumeBackup)
	state.DokployRestart = types.BoolValue(n.DokployRestart)
	state.DockerCleanup = types.BoolValue(n.DockerCleanup)
	state.ServerThreshold = types.BoolValue(n.ServerThreshold)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *slackNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Task 1's API.md verification told us whether updates are universal or
	// type-specific. If type-specific, call r.client.UpdateSlackNotification(...).
	// If universal, call r.client.UpdateNotification(...).
	// As a safe default, Update simply re-creates: the only way an update can
	// happen if it isn't supported is via destroy+create, which Terraform handles
	// via RequiresReplace. Apply this fallback only if Task 1 found no update
	// endpoint:
	resp.Diagnostics.AddError("Update not implemented", "Notification updates require an update endpoint in the Dokploy API. If your version supports it, extend this resource to call r.client.UpdateSlackNotification(...).")
}

func (r *slackNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state slackNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNotification(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting slack notification", err.Error())
	}
}

func (r *slackNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

> **Important — Update endpoint:** if Task 1 confirmed `notification.updateSlack` exists, implement Update by calling `r.client.UpdateSlackNotification(...)` instead of returning the error above. If no update endpoint exists, mark every editable attribute as `RequiresReplace()` (in the Schema) so changes cause destroy+create — and remove the error from Update.

Also create `internal/provider/slack_notification_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSlackNotificationResource(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_slack_notification" "test" {
  name        = "tf-acc-slack-%d"
  webhook_url = "https://hooks.slack.com/services/T0/B0/Xfake"
  channel     = "#tf-acc-tests"
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_slack_notification.test", "id"),
					resource.TestCheckResourceAttr("dokploy_slack_notification.test", "channel", "#tf-acc-tests"),
					resource.TestCheckResourceAttr("dokploy_slack_notification.test", "app_deploy", "true"),
				),
			},
			{
				ResourceName:            "dokploy_slack_notification.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"webhook_url"},
			},
		},
	})
}
```

- [ ] **Step 6: Create the four remaining notification resources following the same pattern**

For each of `discord`, `email`, `telegram`, `gotify`:

1. Copy `slack_notification_resource.go` to a new file with the appropriate name.
2. Rename the model struct, constructor, and resource type.
3. Replace `WebhookURL`/`Channel` attributes with the type-specific ones (see schema table below).
4. Replace `SlackNotificationInput` with the matching `<Type>NotificationInput`.
5. In `Create`, call `r.client.Create<Type>Notification(...)`.
6. Write the corresponding acceptance test file.

Per-type schema differences:

**discord_notification** — replace channel block with:
```go
"webhook_url": schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "Discord webhook URL."},
"decoration":  schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Enable emoji decoration."},
```

**email_notification** — replace with:
```go
"smtp_server":  schema.StringAttribute{Required: true, MarkdownDescription: "SMTP server hostname."},
"smtp_port":    schema.Int64Attribute{Required: true, MarkdownDescription: "SMTP server port."},
"username":     schema.StringAttribute{Required: true, MarkdownDescription: "SMTP username."},
"password":     schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "SMTP password."},
"from_address": schema.StringAttribute{Required: true, MarkdownDescription: "From email address."},
"to_addresses": schema.ListAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Recipient email addresses."},
```

**telegram_notification** — replace with:
```go
"bot_token":         schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "Telegram bot token."},
"chat_id":           schema.StringAttribute{Required: true, MarkdownDescription: "Telegram chat or group ID."},
"message_thread_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Forum group message thread ID."},
```

**gotify_notification** — replace with:
```go
"server_url": schema.StringAttribute{Required: true, MarkdownDescription: "Gotify server URL."},
"app_token":  schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "Gotify app token."},
"priority":   schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Notification priority (1-10)."},
"decoration": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Enable emoji decoration."},
```

For each acceptance test, use a "fake but Zod-valid" config:
- Discord: `webhook_url = "https://discord.com/api/webhooks/0/Xfake"`
- Email: `smtp_server = "smtp.example.com"`, `smtp_port = 587`, etc. (real-looking but won't actually send)
- Telegram: `bot_token = "0:fake"`, `chat_id = "123"`
- Gotify: `server_url = "https://gotify.example.com"`, `app_token = "Afake"`

- [ ] **Step 7: Register all 5 in the provider**

Append to `internal/provider/provider.go::Resources()`:

```go
NewSlackNotificationResource,
NewDiscordNotificationResource,
NewEmailNotificationResource,
NewTelegramNotificationResource,
NewGotifyNotificationResource,
```

- [ ] **Step 8: Build + run all 5 acceptance tests**

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run "TestAcc.+NotificationResource" -v -timeout 30m
```

Expected: all 5 PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/client/notification.go internal/client/notification_test.go \
        internal/provider/slack_notification_resource.go internal/provider/slack_notification_resource_test.go \
        internal/provider/discord_notification_resource.go internal/provider/discord_notification_resource_test.go \
        internal/provider/email_notification_resource.go internal/provider/email_notification_resource_test.go \
        internal/provider/telegram_notification_resource.go internal/provider/telegram_notification_resource_test.go \
        internal/provider/gotify_notification_resource.go internal/provider/gotify_notification_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: dokploy_{slack,discord,email,telegram,gotify}_notification resources"
```

---

## Task 6: dokploy_application advanced config

> **Important Task 1 finding:** `databaseType: "compose"` is **NOT** in the backup Zod enum. Compose backup support is **dropped** from v0.5 — skip Step 2 (do not modify `backup_resource.go` validators, do not modify `client/backup.go`'s `SetTypedID` or `listBackupsForResource`). The web-server backup limitation from v0.3 also cannot be resolved (`application.one` has no `backups` field). Both confirmed; document in v0.5 README's known-limitations section.

> **Important Task 1 finding:** `healthCheckSwarm` and `restartPolicySwarm` use **PascalCase** keys and **nanosecond int64** durations (not Go-style strings). The plan code below shows string fields for ergonomic HCL — the implementer converts strings ("30s") to nanoseconds via `time.ParseDuration(...).Nanoseconds()` before sending, and converts API responses back via `time.Duration(ns).String()` on read.

This task extends `dokploy_application` with `replicas`, `health_check`, and `restart_policy`.

**Files:**
- Modify: `internal/client/application.go` (add HealthCheckSwarm, RestartPolicySwarm, Replicas to struct + Input)
- Modify: `internal/client/backup.go` (SetTypedID handles "compose"; listBackupsForResource handles "compose")
- Modify: `internal/provider/application_resource.go` (new attributes + nested blocks)
- Modify: `internal/provider/application_resource_test.go` (new TestAccApplicationResource_Advanced)
- Modify: `internal/provider/backup_resource.go` (extend `OneOf` to include "compose")

- [ ] **Step 1: Extend `internal/client/application.go`**

Append three new fields to the `Application` struct (right before the closing brace):

```go
	Replicas           *int               `json:"replicas"`
	HealthCheckSwarm   *HealthCheckSwarm  `json:"healthCheckSwarm"`
	RestartPolicySwarm *RestartPolicySwarm `json:"restartPolicySwarm"`
```

Append the same fields to `ApplicationInput`:

```go
	Replicas           *int               `json:"replicas,omitempty"`
	HealthCheckSwarm   *HealthCheckSwarm  `json:"healthCheckSwarm,omitempty"`
	RestartPolicySwarm *RestartPolicySwarm `json:"restartPolicySwarm,omitempty"`
```

Define the two nested types at the bottom of `application.go`:

```go
// HealthCheckSwarm mirrors Docker Swarm's HealthCheck object.
// Durations are nanosecond integers (verified in Task 1 against the live API).
// The provider converts user-facing Go-style strings ("30s") to nanoseconds
// before populating these fields.
type HealthCheckSwarm struct {
	Test        []string `json:"Test,omitempty"`
	Interval    int64    `json:"Interval,omitempty"`    // nanoseconds
	Timeout     int64    `json:"Timeout,omitempty"`     // nanoseconds
	Retries     int      `json:"Retries,omitempty"`
	StartPeriod int64    `json:"StartPeriod,omitempty"` // nanoseconds
}

// RestartPolicySwarm mirrors Docker Swarm's RestartPolicy object.
// Durations are nanosecond integers (same conversion rule as HealthCheckSwarm).
type RestartPolicySwarm struct {
	Condition   string `json:"Condition,omitempty"`
	Delay       int64  `json:"Delay,omitempty"`  // nanoseconds
	MaxAttempts int    `json:"MaxAttempts,omitempty"`
	Window      int64  `json:"Window,omitempty"` // nanoseconds
}
```

> If Task 1 found that the API expects nanosecond integers rather than Go-style duration strings, change the `Interval`/`Timeout`/`Delay`/etc fields to `int64` (nanoseconds). The plan code assumes Go-style; the implementer corrects post-verify.

- [ ] **Step 2: Update `dokploy_backup` to accept `compose`**

In `internal/provider/backup_resource.go`'s Schema method, extend the `OneOf` validator on `database_type`:

```go
Validators: []validator.String{
	stringvalidator.OneOf("postgres", "mysql", "mariadb", "mongo", "web-server", "compose"),
},
```

In `internal/client/backup.go::SetTypedID`, add a case for `compose`:

```go
case "compose":
	// The Compose struct doesn't have a typed id slot in BackupInput because the
	// backup endpoint expects composeId. Add ComposeID field if needed.
```

If `Backup` and `BackupInput` need a new `ComposeID *string` field (verified in Task 1), add it there with `json:"composeId"`. Then:

```go
case "compose":
	in.ComposeID = &id
```

In `listBackupsForResource`, replace the existing `case "web-server"` error stub with handling for both web-server (if Task 9 of Task 1 found the field) and compose:

```go
case "compose":
	co, err := c.GetCompose(ctx, id)
	if err != nil { return nil, err }
	return co.Backups, nil
```

Add `Backups []Backup `json:"backups"`` to the `Compose` struct in `compose.go` (and to `Application` if Task 1 found the field).

- [ ] **Step 3: Add `replicas` + nested blocks to `dokploy_application` schema**

In `internal/provider/application_resource.go`:

Add three fields to `applicationModel`:

```go
	Replicas         types.Int64  `tfsdk:"replicas"`
	HealthCheck      types.Object `tfsdk:"health_check"`
	RestartPolicy    types.Object `tfsdk:"restart_policy"`
```

Add three entries to the schema:

```go
"replicas": schema.Int64Attribute{
	Optional:            true,
	Computed:            true,
	MarkdownDescription: "Number of Docker Swarm replicas. Defaults to whatever Dokploy uses (typically 1).",
	PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
},

"health_check": schema.SingleNestedBlock{
	MarkdownDescription: "Docker Swarm healthcheck. Omit to skip.",
	Attributes: map[string]schema.Attribute{
		"test":         schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Command (e.g. `[\"CMD\", \"curl\", \"-f\", \"http://localhost/health\"]`)."},
		"interval":     schema.StringAttribute{Optional: true, MarkdownDescription: "Probe interval (e.g. `30s`)."},
		"timeout":      schema.StringAttribute{Optional: true, MarkdownDescription: "Probe timeout."},
		"retries":      schema.Int64Attribute{Optional: true, MarkdownDescription: "Number of retries before unhealthy."},
		"start_period": schema.StringAttribute{Optional: true, MarkdownDescription: "Initial grace period."},
	},
},

"restart_policy": schema.SingleNestedBlock{
	MarkdownDescription: "Docker Swarm restart policy. Omit to skip.",
	Attributes: map[string]schema.Attribute{
		"condition":    schema.StringAttribute{Optional: true, MarkdownDescription: "`none`, `on-failure`, or `any`."},
		"delay":        schema.StringAttribute{Optional: true, MarkdownDescription: "Delay between attempts (e.g. `5s`)."},
		"max_attempts": schema.Int64Attribute{Optional: true, MarkdownDescription: "Maximum restart attempts."},
		"window":       schema.StringAttribute{Optional: true, MarkdownDescription: "Evaluation window for max attempts."},
	},
},
```

> Note: `health_check` and `restart_policy` are **blocks**, not attributes — they go under `Blocks: map[string]schema.Block{...}` instead of `Attributes`. Move the existing `timeouts` block alongside them.

Add import for `int64planmodifier`:

```go
"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
```

- [ ] **Step 4: Plumb the new fields through Create/Read/Update**

The plan code is verbose; the implementer writes two helpers and uses them in three CRUD paths.

Add the two helpers near `applicationModel`:

```go
func healthCheckFromModel(ctx context.Context, m types.Object) (*client.HealthCheckSwarm, diag.Diagnostics) {
	if m.IsNull() || m.IsUnknown() {
		return nil, nil
	}
	var inner struct {
		Test        types.List   `tfsdk:"test"`
		Interval    types.String `tfsdk:"interval"`
		Timeout     types.String `tfsdk:"timeout"`
		Retries     types.Int64  `tfsdk:"retries"`
		StartPeriod types.String `tfsdk:"start_period"`
	}
	diags := m.As(ctx, &inner, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	test := []string{}
	if !inner.Test.IsNull() && !inner.Test.IsUnknown() {
		diags.Append(inner.Test.ElementsAs(ctx, &test, false)...)
	}
	return &client.HealthCheckSwarm{
		Test:        test,
		Interval:    inner.Interval.ValueString(),
		Timeout:     inner.Timeout.ValueString(),
		Retries:     int(inner.Retries.ValueInt64()),
		StartPeriod: inner.StartPeriod.ValueString(),
	}, diags
}

func restartPolicyFromModel(ctx context.Context, m types.Object) (*client.RestartPolicySwarm, diag.Diagnostics) {
	if m.IsNull() || m.IsUnknown() {
		return nil, nil
	}
	var inner struct {
		Condition   types.String `tfsdk:"condition"`
		Delay       types.String `tfsdk:"delay"`
		MaxAttempts types.Int64  `tfsdk:"max_attempts"`
		Window      types.String `tfsdk:"window"`
	}
	diags := m.As(ctx, &inner, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	return &client.RestartPolicySwarm{
		Condition:   inner.Condition.ValueString(),
		Delay:       inner.Delay.ValueString(),
		MaxAttempts: int(inner.MaxAttempts.ValueInt64()),
		Window:      inner.Window.ValueString(),
	}, diags
}
```

Add imports `basetypes`, `diag`.

In `Create` and `Update`, after building the existing `client.ApplicationInput{...}`, before calling the API, populate the three new fields:

```go
hc, diagsHC := healthCheckFromModel(ctx, plan.HealthCheck)
resp.Diagnostics.Append(diagsHC...)
rp, diagsRP := restartPolicyFromModel(ctx, plan.RestartPolicy)
resp.Diagnostics.Append(diagsRP...)
if resp.Diagnostics.HasError() { return }

input := client.ApplicationInput{
	// ... existing fields ...
	HealthCheckSwarm:   hc,
	RestartPolicySwarm: rp,
}
if !plan.Replicas.IsNull() && !plan.Replicas.IsUnknown() {
	v := int(plan.Replicas.ValueInt64())
	input.Replicas = &v
}
```

In `Read`, populate the model from the response:

```go
if app.Replicas != nil {
	state.Replicas = types.Int64Value(int64(*app.Replicas))
} else {
	state.Replicas = types.Int64Null()
}

if app.HealthCheckSwarm != nil {
	testList, _ := types.ListValueFrom(ctx, types.StringType, app.HealthCheckSwarm.Test)
	hcObj, _ := types.ObjectValue(
		map[string]attr.Type{
			"test":         types.ListType{ElemType: types.StringType},
			"interval":     types.StringType,
			"timeout":      types.StringType,
			"retries":      types.Int64Type,
			"start_period": types.StringType,
		},
		map[string]attr.Value{
			"test":         testList,
			"interval":     types.StringValue(app.HealthCheckSwarm.Interval),
			"timeout":      types.StringValue(app.HealthCheckSwarm.Timeout),
			"retries":      types.Int64Value(int64(app.HealthCheckSwarm.Retries)),
			"start_period": types.StringValue(app.HealthCheckSwarm.StartPeriod),
		},
	)
	state.HealthCheck = hcObj
} else {
	state.HealthCheck = types.ObjectNull(map[string]attr.Type{
		"test":         types.ListType{ElemType: types.StringType},
		"interval":     types.StringType,
		"timeout":      types.StringType,
		"retries":      types.Int64Type,
		"start_period": types.StringType,
	})
}

if app.RestartPolicySwarm != nil {
	rpObj, _ := types.ObjectValue(
		map[string]attr.Type{
			"condition":    types.StringType,
			"delay":        types.StringType,
			"max_attempts": types.Int64Type,
			"window":       types.StringType,
		},
		map[string]attr.Value{
			"condition":    types.StringValue(app.RestartPolicySwarm.Condition),
			"delay":        types.StringValue(app.RestartPolicySwarm.Delay),
			"max_attempts": types.Int64Value(int64(app.RestartPolicySwarm.MaxAttempts)),
			"window":       types.StringValue(app.RestartPolicySwarm.Window),
		},
	)
	state.RestartPolicy = rpObj
} else {
	state.RestartPolicy = types.ObjectNull(map[string]attr.Type{
		"condition":    types.StringType,
		"delay":        types.StringType,
		"max_attempts": types.Int64Type,
		"window":       types.StringType,
	})
}
```

- [ ] **Step 5: Add the failing acceptance test**

Append to `internal/provider/application_resource_test.go`:

```go
func TestAccApplicationResource_Advanced(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-app-adv-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-app-adv"
  docker_image   = "nginx:alpine"
  replicas       = 2

  health_check {
    test         = ["CMD", "curl", "-f", "http://localhost/"]
    interval     = "30s"
    timeout      = "10s"
    retries      = 3
    start_period = "60s"
  }

  restart_policy {
    condition    = "on-failure"
    delay        = "5s"
    max_attempts = 3
    window       = "120s"
  }

  timeouts { create = "15m" update = "15m" }
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_application.test", "replicas", "2"),
					resource.TestCheckResourceAttr("dokploy_application.test", "health_check.retries", "3"),
					resource.TestCheckResourceAttr("dokploy_application.test", "restart_policy.condition", "on-failure"),
				),
			},
		},
	})
}
```

- [ ] **Step 6: Build + run acceptance**

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run "TestAccApplicationResource_Advanced|TestAccApplicationResource$" -v -timeout 30m
```

Expected: both PASS (no regression in basic application test).

- [ ] **Step 7: Commit**

```bash
git add internal/client/application.go internal/client/backup.go internal/client/compose.go \
        internal/provider/application_resource.go internal/provider/application_resource_test.go \
        internal/provider/backup_resource.go
git commit -m "feat: replicas/healthcheck/restart_policy on dokploy_application; compose backups"
```

---

## Task 7: Examples, README, generated docs

**Files:**
- Create: `examples/resources/dokploy_compose/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_mount/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_port/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_slack_notification/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_discord_notification/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_email_notification/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_telegram_notification/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_gotify_notification/resource.tf` + `import.sh`
- Modify: `README.md`
- Regenerated: `docs/resources/*.md`

- [ ] **Step 1: Create example files**

`examples/resources/dokploy_compose/resource.tf`:

```hcl
resource "dokploy_compose" "monitoring" {
  environment_id = dokploy_project.obs.production_environment_id
  name           = "monitoring"

  compose_file = <<-EOT
    version: "3.8"
    services:
      prometheus:
        image: prom/prometheus:latest
        restart: unless-stopped
  EOT

  env = {
    PROM_PORT = "9090"
  }
}
```

`examples/resources/dokploy_compose/import.sh`:

```bash
terraform import dokploy_compose.monitoring <composeId>
```

`examples/resources/dokploy_mount/resource.tf`:

```hcl
# Bind mount: host path → container path.
resource "dokploy_mount" "static" {
  service_id = dokploy_application.web.id
  type       = "bind"
  mount_path = "/srv/static"
  host_path  = "/var/www/static"
}

# Named volume.
resource "dokploy_mount" "data" {
  service_id  = dokploy_postgres.db.id
  type        = "volume"
  mount_path  = "/var/lib/postgresql/data"
  volume_name = "pg-data"
}

# File inline (config injection).
resource "dokploy_mount" "config" {
  service_id = dokploy_application.web.id
  type       = "file"
  mount_path = "/etc/nginx/conf.d/extra.conf"
  content    = "client_max_body_size 100M;\n"
}
```

`examples/resources/dokploy_mount/import.sh`:

```bash
terraform import dokploy_mount.data <mountId>
```

`examples/resources/dokploy_port/resource.tf`:

```hcl
resource "dokploy_port" "metrics" {
  application_id  = dokploy_application.api.id
  published_port  = 9090
  target_port     = 9090
}
```

`examples/resources/dokploy_port/import.sh`:

```bash
terraform import dokploy_port.metrics <portId>
```

`examples/resources/dokploy_slack_notification/resource.tf`:

```hcl
resource "dokploy_slack_notification" "alerts" {
  name        = "production-alerts"
  webhook_url = var.slack_webhook
  channel     = "#deploys"
}
```

`examples/resources/dokploy_slack_notification/import.sh`:

```bash
terraform import dokploy_slack_notification.alerts <notificationId>
```

`examples/resources/dokploy_discord_notification/resource.tf`:

```hcl
resource "dokploy_discord_notification" "alerts" {
  name        = "production-alerts"
  webhook_url = var.discord_webhook
  decoration  = true
}
```

`examples/resources/dokploy_discord_notification/import.sh`:

```bash
terraform import dokploy_discord_notification.alerts <notificationId>
```

`examples/resources/dokploy_email_notification/resource.tf`:

```hcl
resource "dokploy_email_notification" "alerts" {
  name         = "ops-team"
  smtp_server  = "smtp.example.com"
  smtp_port    = 587
  username     = var.smtp_user
  password     = var.smtp_password
  from_address = "alerts@example.com"
  to_addresses = ["ops@example.com"]
}
```

`examples/resources/dokploy_email_notification/import.sh`:

```bash
terraform import dokploy_email_notification.alerts <notificationId>
```

`examples/resources/dokploy_telegram_notification/resource.tf`:

```hcl
resource "dokploy_telegram_notification" "alerts" {
  name      = "telegram-ops"
  bot_token = var.telegram_bot_token
  chat_id   = var.telegram_chat_id
}
```

`examples/resources/dokploy_telegram_notification/import.sh`:

```bash
terraform import dokploy_telegram_notification.alerts <notificationId>
```

`examples/resources/dokploy_gotify_notification/resource.tf`:

```hcl
resource "dokploy_gotify_notification" "alerts" {
  name       = "gotify-ops"
  server_url = "https://gotify.example.com"
  app_token  = var.gotify_app_token
  priority   = 5
}
```

`examples/resources/dokploy_gotify_notification/import.sh`:

```bash
terraform import dokploy_gotify_notification.alerts <notificationId>
```

- [ ] **Step 2: Update `README.md`**

Find the line `- \`dokploy_host_schedule\` — cron command on the Dokploy host` (the existing v0.3 entry — v0.4 added three more after it but v0.5 lines go after the v0.4 server lines). After `- \`dokploy_server_schedule\` — cron command on a managed server`, insert these eight lines, before `## Data sources`:

```markdown
- `dokploy_compose` — Docker Compose stack
- `dokploy_mount` — bind/volume/file mount on a service
- `dokploy_port` — published port on an application
- `dokploy_slack_notification` — Slack notification
- `dokploy_discord_notification` — Discord notification
- `dokploy_email_notification` — Email (SMTP) notification
- `dokploy_telegram_notification` — Telegram notification
- `dokploy_gotify_notification` — Gotify notification
```

- [ ] **Step 3: Regenerate documentation**

Run: `go generate ./...`
Expected: 8 new files under `docs/resources/`. Existing `application.md` regenerates to include the new `replicas`, `health_check`, and `restart_policy`.

- [ ] **Step 4: Verify**

```bash
git status --short
go build ./...
go vet ./...
```

Expected: build/vet clean.

- [ ] **Step 5: Commit**

```bash
gofmt -w .
git add examples README.md docs/
git commit -m "docs: examples, README entries, and generated docs for v0.5"
```

---

## Task 8: Release v0.5.0

**Files:** none.

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

Run the acceptance suite:

```bash
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/... -v -timeout 90m
```

Expected: every test PASS. Confirm the live instance has no `tf-acc-*` resources left.

- [ ] **Step 2: Merge to master**

```bash
git checkout master
git pull --ff-only
git merge --ff-only <feature-branch>
git push origin master
```

- [ ] **Step 3: Tag and push v0.5.0**

```bash
git tag v0.5.0
git push origin v0.5.0
```

- [ ] **Step 4: Watch the workflow**

```bash
RUN_ID=$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')
gh run watch "$RUN_ID" --exit-status
```

Expected: SUCCESS in ~4 minutes.

- [ ] **Step 5: Verify the GitHub Release**

```bash
gh release view v0.5.0 --json assets -q '.assets[].name' | sort
gh release download v0.5.0 -p '*_SHA256SUMS' -O - | grep manifest
```

Expected: 13 assets named `0.5.0`, and the `SHA256SUMS` line containing `0.5.0_manifest.json`.

- [ ] **Step 6: Confirm Registry picked up v0.5.0**

```bash
/usr/bin/curl -s "https://registry.terraform.io/v1/providers/lucasaarch/dokploy/versions" \
  | /opt/homebrew/bin/python3 -m json.tool | grep -E '"version"' | head -5
```

Expected: a `"version": "0.5.0"` entry. May take ~5 min after the workflow finishes.

---

## Self-review checklist

- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` clean
- [ ] `go test ./internal/client/... -v` passes
- [ ] `go generate ./...` produces no uncommitted diff
- [ ] All 11 new acceptance tests pass (compose, 3 mount variants, port, 5 notifications, application_advanced)
- [ ] All v0.1 + v0.2 + v0.3 + v0.4 acceptance tests still pass (no regression)
- [ ] Live instance clean of `tf-acc-*` resources after the suite
- [ ] All eight new resources registered in `internal/provider/provider.go`'s `Resources()` list
- [ ] `dokploy_backup.database_type` validator accepts `"compose"`
- [ ] `v0.5.0` tag pushed and GitHub Release published with 13 signed assets
- [ ] Registry shows `0.5.0`
