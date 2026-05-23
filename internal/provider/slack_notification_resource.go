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
	SlackID         types.String `tfsdk:"slack_id"`
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
			"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"slack_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Slack-specific sub-ID used for updates.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":             schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"webhook_url":      schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "Slack incoming webhook URL."},
			"channel":          schema.StringAttribute{Required: true, MarkdownDescription: "Slack channel (e.g. `#deploys`)."},
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
	// Extract the slackId from the notification response
	if n.SlackID != nil {
		plan.SlackID = types.StringValue(*n.SlackID)
	} else if n.Slack != nil {
		plan.SlackID = types.StringValue(n.Slack.SlackID)
	} else {
		plan.SlackID = types.StringNull()
	}
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
	// Update slackId
	if n.SlackID != nil {
		state.SlackID = types.StringValue(*n.SlackID)
	} else if n.Slack != nil {
		state.SlackID = types.StringValue(n.Slack.SlackID)
	}
	// Webhook URL returned in plaintext — update from API
	if n.Slack != nil && n.Slack.WebhookURL != "" {
		state.WebhookURL = types.StringValue(n.Slack.WebhookURL)
	}
	if n.Slack != nil {
		state.Channel = types.StringValue(n.Slack.Channel)
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

func (r *slackNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan slackNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Need to read state to get the slackId (type-specific sub-id)
	var state slackNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	slackID := state.SlackID.ValueString()
	if err := r.client.UpdateSlackNotification(ctx, plan.ID.ValueString(), slackID, plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating slack notification", err.Error())
		return
	}
	plan.SlackID = state.SlackID
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
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
