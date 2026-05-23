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
	_ resource.Resource                = &telegramNotificationResource{}
	_ resource.ResourceWithConfigure   = &telegramNotificationResource{}
	_ resource.ResourceWithImportState = &telegramNotificationResource{}
)

type telegramNotificationResource struct{ client *client.Client }

func NewTelegramNotificationResource() resource.Resource { return &telegramNotificationResource{} }

type telegramNotificationModel struct {
	ID              types.String `tfsdk:"id"`
	TelegramID      types.String `tfsdk:"telegram_id"`
	Name            types.String `tfsdk:"name"`
	BotToken        types.String `tfsdk:"bot_token"`
	ChatID          types.String `tfsdk:"chat_id"`
	MessageThreadID types.String `tfsdk:"message_thread_id"`
	AppDeploy       types.Bool   `tfsdk:"app_deploy"`
	AppBuildError   types.Bool   `tfsdk:"app_build_error"`
	DatabaseBackup  types.Bool   `tfsdk:"database_backup"`
	DokployBackup   types.Bool   `tfsdk:"dokploy_backup"`
	VolumeBackup    types.Bool   `tfsdk:"volume_backup"`
	DokployRestart  types.Bool   `tfsdk:"dokploy_restart"`
	DockerCleanup   types.Bool   `tfsdk:"docker_cleanup"`
	ServerThreshold types.Bool   `tfsdk:"server_threshold"`
}

func (r *telegramNotificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_telegram_notification"
}

func (r *telegramNotificationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Telegram notification configuration.",
		Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"telegram_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "Telegram-specific sub-ID used for updates.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":              schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"bot_token":         schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "Telegram bot token."},
			"chat_id":           schema.StringAttribute{Required: true, MarkdownDescription: "Telegram chat or group ID."},
			"message_thread_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Forum group message thread ID."},
			"app_deploy":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on application deploy."},
			"app_build_error":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on build error."},
			"database_backup":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on database backup events."},
			"dokploy_backup":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on Dokploy self-backup events."},
			"volume_backup":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on volume backup events."},
			"dokploy_restart":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on Dokploy restart."},
			"docker_cleanup":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on Docker cleanup."},
			"server_threshold":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Notify on server resource threshold breaches."},
		},
	}
}

func (r *telegramNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m telegramNotificationModel) toInput() client.TelegramNotificationInput {
	return client.TelegramNotificationInput{
		Name:            m.Name.ValueString(),
		BotToken:        m.BotToken.ValueString(),
		ChatID:          m.ChatID.ValueString(),
		MessageThreadID: m.MessageThreadID.ValueString(),
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

func (r *telegramNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan telegramNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n, err := r.client.CreateTelegramNotification(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating telegram notification", err.Error())
		return
	}
	plan.ID = types.StringValue(n.ID)
	if n.TelegramID != nil {
		plan.TelegramID = types.StringValue(*n.TelegramID)
	} else if n.Telegram != nil {
		plan.TelegramID = types.StringValue(n.Telegram.TelegramID)
	} else {
		plan.TelegramID = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *telegramNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state telegramNotificationModel
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
		resp.Diagnostics.AddError("Error reading telegram notification", err.Error())
		return
	}
	state.Name = types.StringValue(n.Name)
	if n.TelegramID != nil {
		state.TelegramID = types.StringValue(*n.TelegramID)
	} else if n.Telegram != nil {
		state.TelegramID = types.StringValue(n.Telegram.TelegramID)
	}
	if n.Telegram != nil {
		if n.Telegram.BotToken != "" {
			state.BotToken = types.StringValue(n.Telegram.BotToken)
		}
		state.ChatID = types.StringValue(n.Telegram.ChatID)
		if n.Telegram.MessageThreadID != "" {
			state.MessageThreadID = types.StringValue(n.Telegram.MessageThreadID)
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

func (r *telegramNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan telegramNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state telegramNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	telegramID := state.TelegramID.ValueString()
	if err := r.client.UpdateTelegramNotification(ctx, plan.ID.ValueString(), telegramID, plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating telegram notification", err.Error())
		return
	}
	plan.TelegramID = state.TelegramID
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *telegramNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state telegramNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNotification(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting telegram notification", err.Error())
	}
}

func (r *telegramNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
