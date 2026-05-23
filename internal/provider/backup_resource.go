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
		MarkdownDescription: "A scheduled backup of a Dokploy database or application. Stores artefacts in a `dokploy_destination`.\n\n**Limitation:** `database_type = \"web-server\"` (application backups) is not supported in this provider version because `application.one` does not return a `backups[]` field, making the post-create ID discovery impossible.",
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
	if b.Enabled != nil {
		plan.Enabled = types.BoolValue(*b.Enabled)
	} else {
		plan.Enabled = types.BoolNull()
	}
	if b.KeepLatestCount != nil {
		plan.KeepLatestCount = types.Int64Value(int64(*b.KeepLatestCount))
	} else {
		plan.KeepLatestCount = types.Int64Null()
	}
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
	if b.Enabled != nil {
		state.Enabled = types.BoolValue(*b.Enabled)
	} else {
		state.Enabled = types.BoolNull()
	}
	if b.KeepLatestCount != nil {
		state.KeepLatestCount = types.Int64Value(int64(*b.KeepLatestCount))
	} else {
		state.KeepLatestCount = types.Int64Null()
	}
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
	// Re-read to populate computed fields (enabled, keep_latest_count) after update.
	b, err := r.client.GetBackup(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading backup after update", err.Error())
		return
	}
	plan.Schedule = types.StringValue(b.Schedule)
	plan.Prefix = types.StringValue(b.Prefix)
	plan.DestinationID = types.StringValue(b.DestinationID)
	if b.Enabled != nil {
		plan.Enabled = types.BoolValue(*b.Enabled)
	} else {
		plan.Enabled = types.BoolNull()
	}
	if b.KeepLatestCount != nil {
		plan.KeepLatestCount = types.Int64Value(int64(*b.KeepLatestCount))
	} else {
		plan.KeepLatestCount = types.Int64Null()
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
