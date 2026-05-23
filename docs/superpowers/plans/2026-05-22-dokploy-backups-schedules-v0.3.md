# Dokploy Backups & Schedules (v0.3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add four managed resources (`dokploy_destination`, `dokploy_backup`, `dokploy_application_schedule`, `dokploy_host_schedule`) so the provider can manage S3-compatible destinations, scheduled backups of databases and applications, and cron commands on app containers or the Dokploy host. Ship as v0.3.0.

**Architecture:** Each resource is a thin Terraform layer wrapping a typed client in `internal/client/<router>.go`. There is no deploy lifecycle for any of these resources — they are pure CRUD configurations on the Dokploy API. The two schedule resources share a single client file (`schedule.go`); they differ only in the payload they send.

**Tech Stack:** Go 1.26, `terraform-plugin-framework`, `terraform-plugin-framework-validators` (for `stringvalidator.OneOf` on the backup discriminator). No new external dependencies.

**Spec:** `docs/superpowers/specs/2026-05-22-dokploy-backups-schedules-v0.3-design.md`

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

## Task 1: Verify destination/backup/schedule routers against the live API

Exploratory probe to fill the three known gaps before any code is written:
1. Exact enum of `destination.provider`.
2. Optional fields of `backup.create` and shape of `backup.one`/`backup.update`.
3. Confirm `schedule.update` endpoint name (we already know `.create` and `.delete` exist; `.update` is assumed).

**Files:**
- Modify: `internal/client/API.md`

- [ ] **Step 1: Load credentials**

```bash
cd /Users/lukearch/Projects/My/dokploy-terraform-provider
source .dokploy-test-env
```

- [ ] **Step 2: Probe destination.create for the provider enum**

Send an intentionally invalid `provider` value — Zod's `expected one of "..."` response reveals the full enum.

```bash
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d '{"name":"tf-probe","provider":"INVALID","bucket":"x","endpoint":"http://x","accessKey":"x","secretAccessKey":"x"}' \
  "$DOKPLOY_ENDPOINT/api/destination.create" | /opt/homebrew/bin/python3 -m json.tool
```

Record the exact list of accepted values (including capitalization).

- [ ] **Step 3: Probe destination.create for the full required-fields set**

```bash
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d '{}' "$DOKPLOY_ENDPOINT/api/destination.create" | /opt/homebrew/bin/python3 -m json.tool
```

Record any fields beyond `name`/`provider`/`bucket`/`endpoint`/`accessKey`/`secretAccessKey` that Zod flags as required.

- [ ] **Step 4: Probe destination.update + destination.one + destination.delete shapes**

Create one throwaway destination (use a fake bucket like `tf-probe-destination`), then:

```bash
# Capture the ID from create response, then:
DEST_ID="<id>"
/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" \
  "$DOKPLOY_ENDPOINT/api/destination.one?destinationId=$DEST_ID" \
  | /opt/homebrew/bin/python3 -m json.tool

# Probe update endpoint name + body shape
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"destinationId\":\"$DEST_ID\",\"name\":\"renamed\"}" \
  "$DOKPLOY_ENDPOINT/api/destination.update" -w "\nHTTP %{http_code}\n"

# Probe delete endpoint name
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"destinationId\":\"$DEST_ID\"}" \
  "$DOKPLOY_ENDPOINT/api/destination.delete" -w "\nHTTP %{http_code}\n"
# If 404, try destination.remove instead.
```

Record: confirmed update path, delete path, and the response shape of `destination.one`.

- [ ] **Step 5: Probe backup.create with all 5 known required + observe optional fields**

```bash
# Use an existing destination and postgres on the instance.
DEST_ID="FwQFgPCZe4wKraiAd_dyd"   # blitz-backups
PG_ID=$(/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" "$DOKPLOY_ENDPOINT/api/project.all" \
  | grep -oE '"postgresId":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "postgres id: $PG_ID"

/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"schedule\":\"0 3 * * *\",\"prefix\":\"tf-probe/\",\"destinationId\":\"$DEST_ID\",\"database\":\"$PG_ID\",\"databaseType\":\"postgres\"}" \
  "$DOKPLOY_ENDPOINT/api/backup.create" -w "\nHTTP %{http_code}\n" \
  | tee /tmp/backup-create.txt

# Grab the backupId from response
BACKUP_ID=$(grep -oE '"backupId":"[^"]*"' /tmp/backup-create.txt | head -1 | cut -d'"' -f4)
echo "backup id: $BACKUP_ID"

# Inspect full backup.one to see what fields exist
/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" \
  "$DOKPLOY_ENDPOINT/api/backup.one?backupId=$BACKUP_ID" \
  | /opt/homebrew/bin/python3 -m json.tool
```

Record the **complete field list** in the `backup.one` response. Specifically look for: `enabled`, `keepLatestCount`, `serviceName`, `database`, `databaseType` — these decide what the resource schema exposes.

- [ ] **Step 6: Probe backup.update + backup.delete endpoint names**

```bash
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"backupId\":\"$BACKUP_ID\",\"schedule\":\"0 4 * * *\"}" \
  "$DOKPLOY_ENDPOINT/api/backup.update" -w "\nHTTP %{http_code}\n"

/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"backupId\":\"$BACKUP_ID\"}" \
  "$DOKPLOY_ENDPOINT/api/backup.delete" -w "\nHTTP %{http_code}\n"
# If 404, try backup.remove.
```

Record confirmed update + delete paths.

- [ ] **Step 7: Probe schedule.update endpoint name**

```bash
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d '{"name":"tf-probe-sched","cronExpression":"0 0 * * *","command":"echo"}' \
  "$DOKPLOY_ENDPOINT/api/schedule.create" | tee /tmp/sched.txt
SCHED_ID=$(grep -oE '"scheduleId":"[^"]*"' /tmp/sched.txt | head -1 | cut -d'"' -f4)

/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"scheduleId\":\"$SCHED_ID\",\"command\":\"echo updated\"}" \
  "$DOKPLOY_ENDPOINT/api/schedule.update" -w "\nHTTP %{http_code}\n"
# If 404, try schedule.edit or schedule.save.
```

Record the confirmed path. Then delete the probe schedule:

```bash
/usr/bin/curl -s -X POST -H "x-api-key: $DOKPLOY_API_KEY" -H "Content-Type: application/json" \
  -d "{\"scheduleId\":\"$SCHED_ID\"}" "$DOKPLOY_ENDPOINT/api/schedule.delete"
```

- [ ] **Step 8: Clean up every probe resource**

Verify no `tf-probe-*` items remain:

```bash
/usr/bin/curl -s -H "x-api-key: $DOKPLOY_API_KEY" "$DOKPLOY_ENDPOINT/api/destination.all" \
  | grep -o '"name":"[^"]*"' | grep tf-probe
# Should print nothing.
```

If any leftover (e.g. destination.update succeeded but renamed instead of cleaning), delete it explicitly.

- [ ] **Step 9: Append three new sections to `internal/client/API.md`**

After the existing sections, append:

- `## destination.*` — methods (create/one/update/delete or update/remove as confirmed), full request body fields (with required vs optional), full response shape of `destination.one`, provider enum values.
- `## backup.*` — methods, request body, response shape, list of fields beyond the 5 required (e.g. `enabled`, `keepLatestCount` if present).
- `## schedule.*` — methods (create/one/<update>/delete), request body (including `scheduleType` enum with all 4 values: `application`/`compose`/`server`/`dokploy-server`), response shape.

- [ ] **Step 10: Commit**

```bash
git add internal/client/API.md
git commit -m "docs: API reference for destination/backup/schedule routers"
```

---

## Task 2: dokploy_destination resource

S3-compatible destination at the organization level. CRUD only — no deploy.

**Files:**
- Create: `internal/client/destination.go`
- Create: `internal/client/destination_test.go`
- Create: `internal/provider/destination_resource.go`
- Create: `internal/provider/destination_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewDestinationResource`)

Adjust field names and endpoint paths to match `API.md` (from Task 1) if anything differs from the plan code.

- [ ] **Step 1: Write the failing client unit tests**

`internal/client/destination_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Destination -v`
Expected: FAIL — `undefined: Destination`, `undefined: DestinationInput`.

- [ ] **Step 3: Write `internal/client/destination.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Destination is an S3-compatible storage configuration at the organization level.
type Destination struct {
	ID               string   `json:"destinationId"`
	Name             string   `json:"name"`
	Provider         string   `json:"provider"`
	Bucket           string   `json:"bucket"`
	Endpoint         string   `json:"endpoint"`
	Region           string   `json:"region"`
	AccessKey        string   `json:"accessKey"`
	SecretAccessKey  string   `json:"secretAccessKey"`
	AdditionalFlags  []string `json:"additionalFlags"`
	OrganizationID   string   `json:"organizationId"`
}

// DestinationInput is the create/update payload.
type DestinationInput struct {
	Name             string   `json:"name,omitempty"`
	Provider         string   `json:"provider,omitempty"`
	Bucket           string   `json:"bucket,omitempty"`
	Endpoint         string   `json:"endpoint,omitempty"`
	Region           string   `json:"region,omitempty"`
	AccessKey        string   `json:"accessKey,omitempty"`
	SecretAccessKey  string   `json:"secretAccessKey,omitempty"`
	AdditionalFlags  []string `json:"additionalFlags,omitempty"`
}

func (c *Client) CreateDestination(ctx context.Context, in DestinationInput) (*Destination, error) {
	var out Destination
	if err := c.do(ctx, http.MethodPost, "destination.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDestination(ctx context.Context, id string) (*Destination, error) {
	var out Destination
	q := url.Values{"destinationId": {id}}
	if err := c.do(ctx, http.MethodGet, "destination.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateDestination(ctx context.Context, id string, in DestinationInput) error {
	payload := struct {
		DestinationInput
		ID string `json:"destinationId"`
	}{DestinationInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "destination.update", payload, nil, nil)
}

func (c *Client) DeleteDestination(ctx context.Context, id string) error {
	payload := map[string]string{"destinationId": id}
	return c.do(ctx, http.MethodPost, "destination.delete", payload, nil, nil)
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Destination -v`
Expected: PASS.

- [ ] **Step 5: Write the failing acceptance test**

`internal/provider/destination_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDestinationResource(t *testing.T) {
	suffix := randInt()
	config := func(name string) string {
		return fmt.Sprintf(`
resource "dokploy_destination" "test" {
  name              = %q
  provider          = "digital_ocean"
  bucket            = "tf-acc-bucket-%d"
  endpoint          = "https://sfo3.digitaloceanspaces.com"
  access_key        = "AKIAEXAMPLEKEY1234"
  secret_access_key = "ExampleSecret1234567890abcdef"
}`, name, suffix)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(fmt.Sprintf("tf-acc-dest-%d", suffix)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_destination.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_destination.test", "organization_id"),
					resource.TestCheckResourceAttr("dokploy_destination.test", "provider", "digital_ocean"),
				),
			},
			{
				ResourceName:      "dokploy_destination.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config(fmt.Sprintf("tf-acc-dest-%d-renamed", suffix)),
				Check:  resource.TestCheckResourceAttr("dokploy_destination.test", "name", fmt.Sprintf("tf-acc-dest-%d-renamed", suffix)),
			},
		},
	})
}
```

> If `provider` collides with Terraform's reserved meta-attribute name, the resource code in Step 7 maps the `tfsdk` tag to a non-conflicting name. The above HCL uses the public attribute name from the schema; if the schema names it `provider_type`, update the HCL to `provider_type = "digital_ocean"`. Decide and lock in during Step 7.

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...`
Expected: FAIL — `undefined: NewDestinationResource`.

- [ ] **Step 7: Write `internal/provider/destination_resource.go`**

> **Important:** `provider` is a reserved word in some Terraform contexts (notably HCL configuration blocks). The framework accepts `provider` as an attribute name, but to avoid surprises this resource exposes it as `provider_type` instead. Adjust the acceptance test in Step 5 to use `provider_type = "digital_ocean"` if you keep this naming.

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
	_ resource.Resource                = &destinationResource{}
	_ resource.ResourceWithConfigure   = &destinationResource{}
	_ resource.ResourceWithImportState = &destinationResource{}
)

type destinationResource struct {
	client *client.Client
}

func NewDestinationResource() resource.Resource { return &destinationResource{} }

type destinationModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	ProviderType     types.String `tfsdk:"provider_type"`
	Bucket           types.String `tfsdk:"bucket"`
	Endpoint         types.String `tfsdk:"endpoint"`
	Region           types.String `tfsdk:"region"`
	AccessKey        types.String `tfsdk:"access_key"`
	SecretAccessKey  types.String `tfsdk:"secret_access_key"`
	AdditionalFlags  types.List   `tfsdk:"additional_flags"`
	OrganizationID   types.String `tfsdk:"organization_id"`
}

func (r *destinationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_destination"
}

func (r *destinationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An S3-compatible storage destination used by `dokploy_backup`. Lives at the organization level.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":            schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"provider_type":   schema.StringAttribute{Required: true, MarkdownDescription: "S3 provider (enum value from the Dokploy API; see API.md).", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"bucket":          schema.StringAttribute{Required: true, MarkdownDescription: "S3 bucket name."},
			"endpoint":        schema.StringAttribute{Required: true, MarkdownDescription: "S3 endpoint URL."},
			"region":          schema.StringAttribute{Optional: true, MarkdownDescription: "S3 region (empty for DO Spaces, required for AWS)."},
			"access_key":      schema.StringAttribute{Required: true, MarkdownDescription: "S3 access key id."},
			"secret_access_key": schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "S3 secret access key."},
			"additional_flags": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Extra flags forwarded to the backup tool (rclone-style).",
			},
			"organization_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization the destination belongs to (set by the API key).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *destinationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// stringListToGo converts a types.List of strings to []string.
func stringListToGo(ctx context.Context, l types.List) ([]string, error) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	out := []string{}
	if diags := l.ElementsAs(ctx, &out, false); diags.HasError() {
		return nil, fmt.Errorf("converting string list")
	}
	return out, nil
}

func (r *destinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan destinationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flags, err := stringListToGo(ctx, plan.AdditionalFlags)
	if err != nil {
		resp.Diagnostics.AddError("Error reading additional_flags", err.Error())
		return
	}

	d, err := r.client.CreateDestination(ctx, client.DestinationInput{
		Name:            plan.Name.ValueString(),
		Provider:        plan.ProviderType.ValueString(),
		Bucket:          plan.Bucket.ValueString(),
		Endpoint:        plan.Endpoint.ValueString(),
		Region:          plan.Region.ValueString(),
		AccessKey:       plan.AccessKey.ValueString(),
		SecretAccessKey: plan.SecretAccessKey.ValueString(),
		AdditionalFlags: flags,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating destination", err.Error())
		return
	}

	plan.ID = types.StringValue(d.ID)
	plan.OrganizationID = types.StringValue(d.OrganizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *destinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state destinationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.client.GetDestination(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading destination", err.Error())
		return
	}
	state.Name = types.StringValue(d.Name)
	state.ProviderType = types.StringValue(d.Provider)
	state.Bucket = types.StringValue(d.Bucket)
	state.Endpoint = types.StringValue(d.Endpoint)
	if d.Region != "" || !state.Region.IsNull() {
		state.Region = types.StringValue(d.Region)
	}
	state.AccessKey = types.StringValue(d.AccessKey)
	state.SecretAccessKey = types.StringValue(d.SecretAccessKey)
	state.OrganizationID = types.StringValue(d.OrganizationID)
	if len(d.AdditionalFlags) > 0 || !state.AdditionalFlags.IsNull() {
		flagsList, diags := types.ListValueFrom(ctx, types.StringType, d.AdditionalFlags)
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.AdditionalFlags = flagsList
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *destinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan destinationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flags, err := stringListToGo(ctx, plan.AdditionalFlags)
	if err != nil {
		resp.Diagnostics.AddError("Error reading additional_flags", err.Error())
		return
	}

	if err := r.client.UpdateDestination(ctx, plan.ID.ValueString(), client.DestinationInput{
		Name:            plan.Name.ValueString(),
		Bucket:          plan.Bucket.ValueString(),
		Endpoint:        plan.Endpoint.ValueString(),
		Region:          plan.Region.ValueString(),
		AccessKey:       plan.AccessKey.ValueString(),
		SecretAccessKey: plan.SecretAccessKey.ValueString(),
		AdditionalFlags: flags,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating destination", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *destinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state destinationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDestination(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting destination", err.Error())
	}
}

func (r *destinationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

Update the HCL in the acceptance test (Step 5) to match the schema: replace every `provider = "digital_ocean"` with `provider_type = "digital_ocean"`.

- [ ] **Step 8: Register the resource, build, run acceptance test**

Open `internal/provider/provider.go` and append `NewDestinationResource,` to the slice returned by `Resources()`.

```bash
go build ./...
gofmt -w .
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run TestAccDestinationResource -v -timeout 10m
```

Expected: build clean; acceptance test PASS (3 steps).

- [ ] **Step 9: Commit**

```bash
git add internal/client/destination.go internal/client/destination_test.go \
        internal/provider/destination_resource.go internal/provider/destination_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: dokploy_destination resource"
```

---

## Task 3: dokploy_backup resource

Unified backup. Supports `database_type` ∈ {`postgres`, `mysql`, `mariadb`, `mongo`, `web-server`}.

**Files:**
- Create: `internal/client/backup.go`
- Create: `internal/client/backup_test.go`
- Create: `internal/provider/backup_resource.go`
- Create: `internal/provider/backup_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewBackupResource`)

- [ ] **Step 1: Write the failing client unit tests**

`internal/client/backup_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateBackup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/backup.create" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body BackupInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DatabaseType != "postgres" {
			t.Errorf("databaseType = %q", body.DatabaseType)
		}
		_ = json.NewEncoder(w).Encode(Backup{
			ID:            "b1",
			Schedule:      body.Schedule,
			Prefix:        body.Prefix,
			DestinationID: body.DestinationID,
			Database:      body.Database,
			DatabaseType:  body.DatabaseType,
			Enabled:       true,
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "k")

	b, err := c.CreateBackup(context.Background(), BackupInput{
		Schedule:      "0 3 * * *",
		Prefix:        "postgres/app/",
		DestinationID: "d1",
		Database:      "pg1",
		DatabaseType:  "postgres",
	})
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	if b.ID != "b1" || b.DatabaseType != "postgres" {
		t.Errorf("b = %+v", b)
	}
}

func TestGetBackup_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetBackup(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Backup -v` — Expected: FAIL.

- [ ] **Step 3: Write `internal/client/backup.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Backup is a scheduled backup configuration.
type Backup struct {
	ID              string `json:"backupId"`
	Schedule        string `json:"schedule"`
	Prefix          string `json:"prefix"`
	DestinationID   string `json:"destinationId"`
	Database        string `json:"database"`
	DatabaseType    string `json:"databaseType"`
	Enabled         bool   `json:"enabled"`
	KeepLatestCount int    `json:"keepLatestCount"`
}

// BackupInput is the create/update payload.
type BackupInput struct {
	Schedule        string `json:"schedule,omitempty"`
	Prefix          string `json:"prefix,omitempty"`
	DestinationID   string `json:"destinationId,omitempty"`
	Database        string `json:"database,omitempty"`
	DatabaseType    string `json:"databaseType,omitempty"`
	Enabled         *bool  `json:"enabled,omitempty"`
	KeepLatestCount *int   `json:"keepLatestCount,omitempty"`
}

func (c *Client) CreateBackup(ctx context.Context, in BackupInput) (*Backup, error) {
	var out Backup
	if err := c.do(ctx, http.MethodPost, "backup.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetBackup(ctx context.Context, id string) (*Backup, error) {
	var out Backup
	q := url.Values{"backupId": {id}}
	if err := c.do(ctx, http.MethodGet, "backup.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateBackup(ctx context.Context, id string, in BackupInput) error {
	payload := struct {
		BackupInput
		ID string `json:"backupId"`
	}{BackupInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "backup.update", payload, nil, nil)
}

func (c *Client) DeleteBackup(ctx context.Context, id string) error {
	payload := map[string]string{"backupId": id}
	return c.do(ctx, http.MethodPost, "backup.delete", payload, nil, nil)
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Backup -v` — Expected: PASS.

- [ ] **Step 5: Write the failing acceptance tests**

`internal/provider/backup_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Helper config: project + postgres + destination + backup against postgres.
func backupPostgresConfig(suffix int, schedule string) string {
	return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-bk-proj-%d"
}

resource "dokploy_postgres" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-bk-pg"
  docker_image   = "postgres:16"
  database_name  = "app"
  database_user  = "app"
  timeouts {
    create = "15m"
    update = "15m"
  }
}

resource "dokploy_destination" "test" {
  name              = "tf-acc-bk-dest-%d"
  provider_type     = "digital_ocean"
  bucket            = "tf-acc-bucket-%d"
  endpoint          = "https://sfo3.digitaloceanspaces.com"
  access_key        = "AKIAEXAMPLEKEY1234"
  secret_access_key = "ExampleSecret1234567890abcdef"
}

resource "dokploy_backup" "test" {
  database_type  = "postgres"
  database_id    = dokploy_postgres.test.id
  destination_id = dokploy_destination.test.id
  schedule       = %q
  prefix         = "tf-acc/postgres/"
}`, suffix, suffix, suffix, schedule)
}

func TestAccBackupResource(t *testing.T) {
	suffix := randInt()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: backupPostgresConfig(suffix, "0 3 * * *"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_backup.test", "id"),
					resource.TestCheckResourceAttr("dokploy_backup.test", "database_type", "postgres"),
					resource.TestCheckResourceAttr("dokploy_backup.test", "schedule", "0 3 * * *"),
				),
			},
			{
				ResourceName:      "dokploy_backup.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: backupPostgresConfig(suffix, "0 4 * * *"),
				Check:  resource.TestCheckResourceAttr("dokploy_backup.test", "schedule", "0 4 * * *"),
			},
		},
	})
}

func TestAccBackup_WebServer(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-bk-app-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-bk-app"
  docker_image   = "nginx:1.27"
  timeouts {
    create = "15m"
    update = "15m"
  }
}

resource "dokploy_destination" "test" {
  name              = "tf-acc-bk-app-dest-%d"
  provider_type     = "digital_ocean"
  bucket            = "tf-acc-bucket-%d"
  endpoint          = "https://sfo3.digitaloceanspaces.com"
  access_key        = "AKIAEXAMPLEKEY1234"
  secret_access_key = "ExampleSecret1234567890abcdef"
}

resource "dokploy_backup" "test" {
  database_type  = "web-server"
  database_id    = dokploy_application.test.id
  destination_id = dokploy_destination.test.id
  schedule       = "0 5 * * 0"
  prefix         = "tf-acc/web-server/"
}`, suffix, suffix, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_backup.test", "id"),
					resource.TestCheckResourceAttr("dokploy_backup.test", "database_type", "web-server"),
				),
			},
		},
	})
}
```

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewBackupResource`.

- [ ] **Step 7: Write `internal/provider/backup_resource.go`**

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
	_ resource.Resource                = &backupResource{}
	_ resource.ResourceWithConfigure   = &backupResource{}
	_ resource.ResourceWithImportState = &backupResource{}
)

type backupResource struct {
	client *client.Client
}

func NewBackupResource() resource.Resource { return &backupResource{} }

type backupModel struct {
	ID              types.String `tfsdk:"id"`
	DatabaseType    types.String `tfsdk:"database_type"`
	DatabaseID      types.String `tfsdk:"database_id"`
	DestinationID   types.String `tfsdk:"destination_id"`
	Schedule        types.String `tfsdk:"schedule"`
	Prefix          types.String `tfsdk:"prefix"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	KeepLatestCount types.Int64  `tfsdk:"keep_latest_count"`
}

func (r *backupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup"
}

func (r *backupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A scheduled backup of a Dokploy database or application. Stores artefacts in a `dokploy_destination`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"database_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "What kind of resource is being backed up. One of `postgres`, `mysql`, `mariadb`, `mongo`, `web-server`.",
				Validators: []validator.String{
					stringvalidator.OneOf("postgres", "mysql", "mariadb", "mongo", "web-server"),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"database_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Identifier of the resource being backed up (`dokploy_postgres.x.id`, `dokploy_application.x.id`, etc).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"destination_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "`dokploy_destination.x.id`.",
			},
			"schedule": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Cron expression (ex: `0 3 * * *`).",
			},
			"prefix": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Path prefix inside the bucket where backup files are written.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the schedule is active. Defaults to `true`.",
			},
			"keep_latest_count": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Retention — keep only the latest N backups. Omit to keep all.",
			},
		},
	}
}

func (r *backupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m backupModel) toInput() client.BackupInput {
	in := client.BackupInput{
		Schedule:      m.Schedule.ValueString(),
		Prefix:        m.Prefix.ValueString(),
		DestinationID: m.DestinationID.ValueString(),
		Database:      m.DatabaseID.ValueString(),
		DatabaseType:  m.DatabaseType.ValueString(),
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		v := m.Enabled.ValueBool()
		in.Enabled = &v
	}
	if !m.KeepLatestCount.IsNull() && !m.KeepLatestCount.IsUnknown() {
		v := int(m.KeepLatestCount.ValueInt64())
		in.KeepLatestCount = &v
	}
	return in
}

func (r *backupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b, err := r.client.CreateBackup(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating backup", err.Error())
		return
	}
	plan.ID = types.StringValue(b.ID)
	plan.Enabled = types.BoolValue(b.Enabled)
	plan.KeepLatestCount = types.Int64Value(int64(b.KeepLatestCount))
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *backupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state backupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b, err := r.client.GetBackup(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading backup", err.Error())
		return
	}
	state.DatabaseType = types.StringValue(b.DatabaseType)
	state.DatabaseID = types.StringValue(b.Database)
	state.DestinationID = types.StringValue(b.DestinationID)
	state.Schedule = types.StringValue(b.Schedule)
	state.Prefix = types.StringValue(b.Prefix)
	state.Enabled = types.BoolValue(b.Enabled)
	state.KeepLatestCount = types.Int64Value(int64(b.KeepLatestCount))
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *backupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan backupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdateBackup(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating backup", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *backupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state backupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteBackup(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting backup", err.Error())
	}
}

func (r *backupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run acceptance tests**

Append `NewBackupResource,` to `Resources()` in `internal/provider/provider.go`.

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run "TestAccBackupResource|TestAccBackup_WebServer" -v -timeout 30m
```

Expected: both tests PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/client/backup.go internal/client/backup_test.go \
        internal/provider/backup_resource.go internal/provider/backup_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: dokploy_backup resource (unified — postgres/mysql/mariadb/mongo/web-server)"
```

---

## Task 4: dokploy_application_schedule resource (with shared schedule client)

This task adds the shared `schedule` client and the first of two schedule resources.

**Files:**
- Create: `internal/client/schedule.go`
- Create: `internal/client/schedule_test.go`
- Create: `internal/provider/application_schedule_resource.go`
- Create: `internal/provider/application_schedule_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewApplicationScheduleResource`)

- [ ] **Step 1: Write failing client unit tests**

`internal/client/schedule_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -run Schedule -v` — Expected: FAIL.

- [ ] **Step 3: Write `internal/client/schedule.go`**

```go
package client

import (
	"context"
	"net/http"
	"net/url"
)

// Schedule is a cron-command configuration.
type Schedule struct {
	ID             string `json:"scheduleId"`
	Name           string `json:"name"`
	CronExpression string `json:"cronExpression"`
	Command        string `json:"command"`
	ShellType      string `json:"shellType"`
	ScheduleType   string `json:"scheduleType"`
	AppName        string `json:"appName"`
	ApplicationID  string `json:"applicationId"`
	ComposeID      string `json:"composeId"`
	ServerID       string `json:"serverId"`
	UserID         string `json:"userId"`
	Enabled        bool   `json:"enabled"`
	Timezone       string `json:"timezone"`
}

// ScheduleInput is the create/update payload.
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

func (c *Client) CreateSchedule(ctx context.Context, in ScheduleInput) (*Schedule, error) {
	var out Schedule
	if err := c.do(ctx, http.MethodPost, "schedule.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	var out Schedule
	q := url.Values{"scheduleId": {id}}
	if err := c.do(ctx, http.MethodGet, "schedule.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateSchedule(ctx context.Context, id string, in ScheduleInput) error {
	payload := struct {
		ScheduleInput
		ID string `json:"scheduleId"`
	}{ScheduleInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "schedule.update", payload, nil, nil)
}

func (c *Client) DeleteSchedule(ctx context.Context, id string) error {
	payload := map[string]string{"scheduleId": id}
	return c.do(ctx, http.MethodPost, "schedule.delete", payload, nil, nil)
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/client/ -run Schedule -v` — Expected: PASS.

- [ ] **Step 5: Write the failing acceptance test**

`internal/provider/application_schedule_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccApplicationScheduleResource(t *testing.T) {
	suffix := randInt()
	config := func(command string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-as-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-as-app"
  docker_image   = "nginx:1.27"
  timeouts {
    create = "15m"
    update = "15m"
  }
}

resource "dokploy_application_schedule" "test" {
  application_id  = dokploy_application.test.id
  name            = "tf-acc-as-sched"
  cron_expression = "0 4 * * *"
  command         = %q
}`, suffix, command)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("echo hello"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_application_schedule.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_application_schedule.test", "app_name"),
					resource.TestCheckResourceAttr("dokploy_application_schedule.test", "command", "echo hello"),
				),
			},
			{
				ResourceName:      "dokploy_application_schedule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config("echo updated"),
				Check:  resource.TestCheckResourceAttr("dokploy_application_schedule.test", "command", "echo updated"),
			},
		},
	})
}
```

- [ ] **Step 6: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewApplicationScheduleResource`.

- [ ] **Step 7: Write `internal/provider/application_schedule_resource.go`**

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
	_ resource.Resource                = &applicationScheduleResource{}
	_ resource.ResourceWithConfigure   = &applicationScheduleResource{}
	_ resource.ResourceWithImportState = &applicationScheduleResource{}
)

type applicationScheduleResource struct {
	client *client.Client
}

func NewApplicationScheduleResource() resource.Resource { return &applicationScheduleResource{} }

type applicationScheduleModel struct {
	ID             types.String `tfsdk:"id"`
	ApplicationID  types.String `tfsdk:"application_id"`
	Name           types.String `tfsdk:"name"`
	CronExpression types.String `tfsdk:"cron_expression"`
	Command        types.String `tfsdk:"command"`
	ShellType      types.String `tfsdk:"shell_type"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Timezone       types.String `tfsdk:"timezone"`
	AppName        types.String `tfsdk:"app_name"`
}

func (r *applicationScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_schedule"
}

func (r *applicationScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A cron-scheduled command run inside a Dokploy application container.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"application_id":  schema.StringAttribute{Required: true, MarkdownDescription: "`dokploy_application.x.id`. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":            schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"cron_expression": schema.StringAttribute{Required: true, MarkdownDescription: "Cron expression (ex: `0 3 * * *`)."},
			"command":         schema.StringAttribute{Required: true, MarkdownDescription: "Shell command to run."},
			"shell_type":      schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Shell used to run the command. Defaults to `bash`.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"enabled":         schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether the schedule is active. Defaults to `true`."},
			"timezone":        schema.StringAttribute{Optional: true, MarkdownDescription: "IANA timezone (ex: `America/Sao_Paulo`). Default null = UTC."},
			"app_name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Internal name generated by Dokploy.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *applicationScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m applicationScheduleModel) toInput() client.ScheduleInput {
	in := client.ScheduleInput{
		Name:           m.Name.ValueString(),
		CronExpression: m.CronExpression.ValueString(),
		Command:        m.Command.ValueString(),
		ScheduleType:   "application",
		ApplicationID:  m.ApplicationID.ValueString(),
		ShellType:      m.ShellType.ValueString(),
		Timezone:       m.Timezone.ValueString(),
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		v := m.Enabled.ValueBool()
		in.Enabled = &v
	}
	return in
}

func (r *applicationScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan applicationScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.CreateSchedule(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating application schedule", err.Error())
		return
	}
	plan.ID = types.StringValue(s.ID)
	plan.AppName = types.StringValue(s.AppName)
	plan.ShellType = types.StringValue(s.ShellType)
	plan.Enabled = types.BoolValue(s.Enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *applicationScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state applicationScheduleModel
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
	state.ApplicationID = types.StringValue(s.ApplicationID)
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

func (r *applicationScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan applicationScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdateSchedule(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating application schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *applicationScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state applicationScheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSchedule(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting schedule", err.Error())
	}
}

func (r *applicationScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register, build, run acceptance test**

Append `NewApplicationScheduleResource,` to `Resources()`.

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run TestAccApplicationScheduleResource -v -timeout 20m
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/client/schedule.go internal/client/schedule_test.go \
        internal/provider/application_schedule_resource.go internal/provider/application_schedule_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: shared schedule client + dokploy_application_schedule resource"
```

---

## Task 5: dokploy_host_schedule resource

Cron on the Dokploy host. Reuses `internal/client/schedule.go` from Task 4.

**Files:**
- Create: `internal/provider/host_schedule_resource.go`
- Create: `internal/provider/host_schedule_resource_test.go`
- Modify: `internal/provider/provider.go` (register `NewHostScheduleResource`)

- [ ] **Step 1: Write the failing acceptance test**

`internal/provider/host_schedule_resource_test.go`:

```go
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccHostScheduleResource(t *testing.T) {
	suffix := randInt()
	config := func(command string) string {
		return fmt.Sprintf(`
resource "dokploy_host_schedule" "test" {
  name            = "tf-acc-hs-%d"
  cron_expression = "0 0 * * *"
  command         = %q
  timezone        = "America/Sao_Paulo"
}`, suffix, command)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("echo hi"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_host_schedule.test", "id"),
					resource.TestCheckResourceAttr("dokploy_host_schedule.test", "command", "echo hi"),
					resource.TestCheckResourceAttr("dokploy_host_schedule.test", "timezone", "America/Sao_Paulo"),
				),
			},
			{
				ResourceName:      "dokploy_host_schedule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config("echo updated"),
				Check:  resource.TestCheckResourceAttr("dokploy_host_schedule.test", "command", "echo updated"),
			},
		},
	})
}
```

- [ ] **Step 2: Verify compile-fail**

Run: `go build ./...` — Expected: `undefined: NewHostScheduleResource`.

- [ ] **Step 3: Write `internal/provider/host_schedule_resource.go`**

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
	_ resource.Resource                = &hostScheduleResource{}
	_ resource.ResourceWithConfigure   = &hostScheduleResource{}
	_ resource.ResourceWithImportState = &hostScheduleResource{}
)

type hostScheduleResource struct {
	client *client.Client
}

func NewHostScheduleResource() resource.Resource { return &hostScheduleResource{} }

type hostScheduleModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	CronExpression types.String `tfsdk:"cron_expression"`
	Command        types.String `tfsdk:"command"`
	ShellType      types.String `tfsdk:"shell_type"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Timezone       types.String `tfsdk:"timezone"`
	AppName        types.String `tfsdk:"app_name"`
}

func (r *hostScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host_schedule"
}

func (r *hostScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A cron-scheduled command run on the Dokploy host (the machine running Dokploy itself). Use this for instance-wide jobs.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
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

func (r *hostScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m hostScheduleModel) toInput() client.ScheduleInput {
	in := client.ScheduleInput{
		Name:           m.Name.ValueString(),
		CronExpression: m.CronExpression.ValueString(),
		Command:        m.Command.ValueString(),
		ScheduleType:   "dokploy-server",
		ShellType:      m.ShellType.ValueString(),
		Timezone:       m.Timezone.ValueString(),
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		v := m.Enabled.ValueBool()
		in.Enabled = &v
	}
	return in
}

func (r *hostScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.CreateSchedule(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating host schedule", err.Error())
		return
	}
	plan.ID = types.StringValue(s.ID)
	plan.AppName = types.StringValue(s.AppName)
	plan.ShellType = types.StringValue(s.ShellType)
	plan.Enabled = types.BoolValue(s.Enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *hostScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostScheduleModel
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

func (r *hostScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan hostScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdateSchedule(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating host schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *hostScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostScheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSchedule(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting schedule", err.Error())
	}
}

func (r *hostScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Register, build, run acceptance test**

Append `NewHostScheduleResource,` to `Resources()`.

```bash
gofmt -w .
go build ./...
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/ -run TestAccHostScheduleResource -v -timeout 10m
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/host_schedule_resource.go internal/provider/host_schedule_resource_test.go \
        internal/provider/provider.go
git commit -m "feat: dokploy_host_schedule resource"
```

---

## Task 6: Examples, README, generated docs

**Files:**
- Create: `examples/resources/dokploy_destination/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_backup/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_application_schedule/resource.tf` + `import.sh`
- Create: `examples/resources/dokploy_host_schedule/resource.tf` + `import.sh`
- Modify: `README.md`
- Regenerated: `docs/resources/*.md`

- [ ] **Step 1: Create example files**

`examples/resources/dokploy_destination/resource.tf`:

```hcl
resource "dokploy_destination" "s3" {
  name              = "prod-backups"
  provider_type     = "digital_ocean"
  bucket            = "my-bucket"
  endpoint          = "https://sfo3.digitaloceanspaces.com"
  access_key        = var.do_access_key
  secret_access_key = var.do_secret_key
}
```

`examples/resources/dokploy_destination/import.sh`:

```bash
terraform import dokploy_destination.s3 <destinationId>
```

`examples/resources/dokploy_backup/resource.tf`:

```hcl
resource "dokploy_backup" "db_daily" {
  database_type  = "postgres"
  database_id    = dokploy_postgres.db.id
  destination_id = dokploy_destination.s3.id
  schedule       = "0 3 * * *"
  prefix         = "postgres/app/"
}
```

`examples/resources/dokploy_backup/import.sh`:

```bash
terraform import dokploy_backup.db_daily <backupId>
```

`examples/resources/dokploy_application_schedule/resource.tf`:

```hcl
resource "dokploy_application_schedule" "warmup" {
  application_id  = dokploy_application.api.id
  name            = "warmup-cache"
  cron_expression = "*/15 * * * *"
  command         = "curl -s http://localhost:3000/internal/warmup"
}
```

`examples/resources/dokploy_application_schedule/import.sh`:

```bash
terraform import dokploy_application_schedule.warmup <scheduleId>
```

`examples/resources/dokploy_host_schedule/resource.tf`:

```hcl
resource "dokploy_host_schedule" "rotate_logs" {
  name            = "rotate-traefik-logs"
  cron_expression = "0 0 * * *"
  command         = "find /var/log/dokploy -name '*.log.*' -mtime +14 -delete"
  timezone        = "America/Sao_Paulo"
}
```

`examples/resources/dokploy_host_schedule/import.sh`:

```bash
terraform import dokploy_host_schedule.rotate_logs <scheduleId>
```

- [ ] **Step 2: Update `README.md`**

Find the line `- \`dokploy_redis\` — managed Redis service` (the last line of the v0.2 resources block in the `## Resources` section). Insert these four lines directly after it, before the `## Data sources` section:

```markdown
- `dokploy_destination` — S3-compatible storage destination (organization-level)
- `dokploy_backup` — scheduled backup of a database or application
- `dokploy_application_schedule` — cron command inside an application container
- `dokploy_host_schedule` — cron command on the Dokploy host
```

- [ ] **Step 3: Regenerate documentation**

Run: `go generate ./...`
Expected: 4 new files in `docs/resources/` (`destination.md`, `backup.md`, `application_schedule.md`, `host_schedule.md`).

- [ ] **Step 4: Verify**

```bash
git status --short
go build ./...
go vet ./...
```

Expected: build/vet clean. Diff shows new examples, README change, 4 new generated docs files. `docs/superpowers/` untouched.

- [ ] **Step 5: Commit**

```bash
gofmt -w .
git add examples README.md docs/
git commit -m "docs: examples, README entries, and generated docs for v0.3 destinations/backups/schedules"
```

---

## Task 7: Release v0.3.0

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

Expected: all green; no uncommitted docs diff. If `go generate` produces a diff, commit it under `docs: regenerate docs`.

Run the full acceptance suite:

```bash
source .dokploy-test-env
TF_ACC=1 go test ./internal/provider/... -v -timeout 90m
```

Expected: every test PASS (v0.1 + v0.2 + 5 new tests). Confirm the live instance has no `tf-acc-*` resources left after the suite.

- [ ] **Step 2: Merge to master**

If on a feature branch:

```bash
git checkout master
git pull --ff-only
git merge --ff-only <feature-branch>
git push origin master
```

- [ ] **Step 3: Tag and push v0.3.0**

```bash
git tag v0.3.0
git push origin v0.3.0
```

The `Release` workflow (`.github/workflows/release.yml`) triggers automatically.

- [ ] **Step 4: Watch the workflow**

```bash
RUN_ID=$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')
gh run watch "$RUN_ID" --exit-status
```

Expected: SUCCESS in ~4 minutes.

- [ ] **Step 5: Verify the GitHub Release**

```bash
gh release view v0.3.0 --json assets -q '.assets[].name' | sort
```

Expected: 13 assets (manifest + SHA256SUMS + SHA256SUMS.sig + 11 platform zips, all named with `0.3.0`).

```bash
gh release download v0.3.0 -p '*_SHA256SUMS' -O - | grep manifest
```

Expected: one line ending in `terraform-provider-dokploy_0.3.0_manifest.json`.

- [ ] **Step 6: Confirm Registry picked up v0.3.0**

```bash
/usr/bin/curl -s "https://registry.terraform.io/v1/providers/lucasaarch/dokploy/versions" \
  | /opt/homebrew/bin/python3 -m json.tool | grep -E '"version"' | head -5
```

Expected: a `"version": "0.3.0"` entry alongside `0.1.0` and `0.2.0`. May take ~5 min after the workflow finishes.

---

## Self-review checklist

Before considering this plan complete, the implementer should confirm:

- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` clean
- [ ] `go test ./internal/client/... -v` passes (new + existing unit tests)
- [ ] `go generate ./...` produces no uncommitted diff
- [ ] All 5 new acceptance tests pass against the live instance
- [ ] All v0.1 + v0.2 acceptance tests still pass (no regression)
- [ ] Live instance clean of `tf-acc-*` resources after the suite
- [ ] All four new resources registered in `internal/provider/provider.go`'s `Resources()` list
- [ ] `v0.3.0` tag pushed and GitHub Release published with 13 signed assets
- [ ] Registry shows `0.3.0`
