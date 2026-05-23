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
	_ resource.Resource                = &discordNotificationResource{}
	_ resource.ResourceWithConfigure   = &discordNotificationResource{}
	_ resource.ResourceWithImportState = &discordNotificationResource{}
)

type discordNotificationResource struct{ client *client.Client }

func NewDiscordNotificationResource() resource.Resource { return &discordNotificationResource{} }

type discordNotificationModel struct {
	ID              types.String `tfsdk:"id"`
	DiscordID       types.String `tfsdk:"discord_id"`
	Name            types.String `tfsdk:"name"`
	WebhookURL      types.String `tfsdk:"webhook_url"`
	Decoration      types.Bool   `tfsdk:"decoration"`
	AppDeploy       types.Bool   `tfsdk:"app_deploy"`
	AppBuildError   types.Bool   `tfsdk:"app_build_error"`
	DatabaseBackup  types.Bool   `tfsdk:"database_backup"`
	DokployBackup   types.Bool   `tfsdk:"dokploy_backup"`
	VolumeBackup    types.Bool   `tfsdk:"volume_backup"`
	DokployRestart  types.Bool   `tfsdk:"dokploy_restart"`
	DockerCleanup   types.Bool   `tfsdk:"docker_cleanup"`
	ServerThreshold types.Bool   `tfsdk:"server_threshold"`
}

func (r *discordNotificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_discord_notification"
}

func (r *discordNotificationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Discord notification configuration.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"discord_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "Discord-specific sub-ID used for updates.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":             schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"webhook_url":      schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "Discord webhook URL."},
			"decoration":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable emoji decoration."},
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

func (r *discordNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m discordNotificationModel) toInput() client.DiscordNotificationInput {
	dec := m.Decoration.ValueBool()
	return client.DiscordNotificationInput{
		Name:       m.Name.ValueString(),
		WebhookURL: m.WebhookURL.ValueString(),
		Decoration: &dec,
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

func (r *discordNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan discordNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n, err := r.client.CreateDiscordNotification(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating discord notification", err.Error())
		return
	}
	plan.ID = types.StringValue(n.ID)
	if n.DiscordID != nil {
		plan.DiscordID = types.StringValue(*n.DiscordID)
	} else if n.Discord != nil {
		plan.DiscordID = types.StringValue(n.Discord.DiscordID)
	} else {
		plan.DiscordID = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *discordNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state discordNotificationModel
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
		resp.Diagnostics.AddError("Error reading discord notification", err.Error())
		return
	}
	state.Name = types.StringValue(n.Name)
	if n.DiscordID != nil {
		state.DiscordID = types.StringValue(*n.DiscordID)
	} else if n.Discord != nil {
		state.DiscordID = types.StringValue(n.Discord.DiscordID)
	}
	if n.Discord != nil && n.Discord.WebhookURL != "" {
		state.WebhookURL = types.StringValue(n.Discord.WebhookURL)
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

func (r *discordNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan discordNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state discordNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	discordID := state.DiscordID.ValueString()
	if err := r.client.UpdateDiscordNotification(ctx, plan.ID.ValueString(), discordID, plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating discord notification", err.Error())
		return
	}
	plan.DiscordID = state.DiscordID
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *discordNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state discordNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNotification(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting discord notification", err.Error())
	}
}

func (r *discordNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
