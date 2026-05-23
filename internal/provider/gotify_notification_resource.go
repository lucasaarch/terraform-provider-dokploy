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
	_ resource.Resource                = &gotifyNotificationResource{}
	_ resource.ResourceWithConfigure   = &gotifyNotificationResource{}
	_ resource.ResourceWithImportState = &gotifyNotificationResource{}
)

type gotifyNotificationResource struct{ client *client.Client }

func NewGotifyNotificationResource() resource.Resource { return &gotifyNotificationResource{} }

type gotifyNotificationModel struct {
	ID              types.String `tfsdk:"id"`
	GotifyID        types.String `tfsdk:"gotify_id"`
	Name            types.String `tfsdk:"name"`
	ServerURL       types.String `tfsdk:"server_url"`
	AppToken        types.String `tfsdk:"app_token"`
	Priority        types.Int64  `tfsdk:"priority"`
	AppDeploy       types.Bool   `tfsdk:"app_deploy"`
	AppBuildError   types.Bool   `tfsdk:"app_build_error"`
	DatabaseBackup  types.Bool   `tfsdk:"database_backup"`
	DokployBackup   types.Bool   `tfsdk:"dokploy_backup"`
	VolumeBackup    types.Bool   `tfsdk:"volume_backup"`
	DokployRestart  types.Bool   `tfsdk:"dokploy_restart"`
	DockerCleanup   types.Bool   `tfsdk:"docker_cleanup"`
	ServerThreshold types.Bool   `tfsdk:"server_threshold"`
}

func (r *gotifyNotificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gotify_notification"
}

func (r *gotifyNotificationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Gotify notification configuration.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"gotify_id":        schema.StringAttribute{Computed: true, MarkdownDescription: "Gotify-specific sub-ID used for updates.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":             schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"server_url":       schema.StringAttribute{Required: true, MarkdownDescription: "Gotify server URL."},
			"app_token":        schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "Gotify app token."},
			"priority":         schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Notification priority (1-10)."},
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

func (r *gotifyNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m gotifyNotificationModel) toInput() client.GotifyNotificationInput {
	var priority *int
	if !m.Priority.IsNull() && !m.Priority.IsUnknown() {
		v := int(m.Priority.ValueInt64())
		priority = &v
	}
	return client.GotifyNotificationInput{
		Name:      m.Name.ValueString(),
		ServerURL: m.ServerURL.ValueString(),
		AppToken:  m.AppToken.ValueString(),
		Priority:  priority,
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

func (r *gotifyNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gotifyNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n, err := r.client.CreateGotifyNotification(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating gotify notification", err.Error())
		return
	}
	plan.ID = types.StringValue(n.ID)
	if n.GotifyID != nil {
		plan.GotifyID = types.StringValue(*n.GotifyID)
	} else if n.Gotify != nil {
		plan.GotifyID = types.StringValue(n.Gotify.GotifyID)
	} else {
		plan.GotifyID = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *gotifyNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gotifyNotificationModel
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
		resp.Diagnostics.AddError("Error reading gotify notification", err.Error())
		return
	}
	state.Name = types.StringValue(n.Name)
	if n.GotifyID != nil {
		state.GotifyID = types.StringValue(*n.GotifyID)
	} else if n.Gotify != nil {
		state.GotifyID = types.StringValue(n.Gotify.GotifyID)
	}
	if n.Gotify != nil {
		state.ServerURL = types.StringValue(n.Gotify.ServerURL)
		if n.Gotify.AppToken != "" {
			state.AppToken = types.StringValue(n.Gotify.AppToken)
		}
		if n.Gotify.Priority != nil {
			state.Priority = types.Int64Value(int64(*n.Gotify.Priority))
		}
	}
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

func (r *gotifyNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gotifyNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state gotifyNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	gotifyID := state.GotifyID.ValueString()
	if err := r.client.UpdateGotifyNotification(ctx, plan.ID.ValueString(), gotifyID, plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating gotify notification", err.Error())
		return
	}
	plan.GotifyID = state.GotifyID
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *gotifyNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gotifyNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNotification(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting gotify notification", err.Error())
	}
}

func (r *gotifyNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
