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
	_ resource.Resource                = &emailNotificationResource{}
	_ resource.ResourceWithConfigure   = &emailNotificationResource{}
	_ resource.ResourceWithImportState = &emailNotificationResource{}
)

type emailNotificationResource struct{ client *client.Client }

func NewEmailNotificationResource() resource.Resource { return &emailNotificationResource{} }

type emailNotificationModel struct {
	ID              types.String `tfsdk:"id"`
	EmailID         types.String `tfsdk:"email_id"`
	Name            types.String `tfsdk:"name"`
	SMTPServer      types.String `tfsdk:"smtp_server"`
	SMTPPort        types.Int64  `tfsdk:"smtp_port"`
	Username        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	FromAddress     types.String `tfsdk:"from_address"`
	ToAddresses     types.List   `tfsdk:"to_addresses"`
	AppDeploy       types.Bool   `tfsdk:"app_deploy"`
	AppBuildError   types.Bool   `tfsdk:"app_build_error"`
	DatabaseBackup  types.Bool   `tfsdk:"database_backup"`
	DokployBackup   types.Bool   `tfsdk:"dokploy_backup"`
	VolumeBackup    types.Bool   `tfsdk:"volume_backup"`
	DokployRestart  types.Bool   `tfsdk:"dokploy_restart"`
	DockerCleanup   types.Bool   `tfsdk:"docker_cleanup"`
	ServerThreshold types.Bool   `tfsdk:"server_threshold"`
}

func (r *emailNotificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_notification"
}

func (r *emailNotificationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An Email (SMTP) notification configuration.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"email_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Email-specific sub-ID used for updates.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":             schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"smtp_server":      schema.StringAttribute{Required: true, MarkdownDescription: "SMTP server hostname."},
			"smtp_port":        schema.Int64Attribute{Required: true, MarkdownDescription: "SMTP server port."},
			"username":         schema.StringAttribute{Required: true, MarkdownDescription: "SMTP username."},
			"password":         schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "SMTP password."},
			"from_address":     schema.StringAttribute{Required: true, MarkdownDescription: "From email address."},
			"to_addresses":     schema.ListAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Recipient email addresses."},
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

func (r *emailNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m emailNotificationModel) toInput(ctx context.Context) (client.EmailNotificationInput, error) {
	var addrs []string
	if diags := m.ToAddresses.ElementsAs(ctx, &addrs, false); diags.HasError() {
		return client.EmailNotificationInput{}, fmt.Errorf("reading to_addresses")
	}
	return client.EmailNotificationInput{
		Name:        m.Name.ValueString(),
		SMTPServer:  m.SMTPServer.ValueString(),
		SMTPPort:    int(m.SMTPPort.ValueInt64()),
		Username:    m.Username.ValueString(),
		Password:    m.Password.ValueString(),
		FromAddress: m.FromAddress.ValueString(),
		ToAddresses: addrs,
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
	}, nil
}

func (r *emailNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan emailNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, err := plan.toInput(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading plan", err.Error())
		return
	}
	n, err := r.client.CreateEmailNotification(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating email notification", err.Error())
		return
	}
	plan.ID = types.StringValue(n.ID)
	if n.EmailID != nil {
		plan.EmailID = types.StringValue(*n.EmailID)
	} else if n.Email != nil {
		plan.EmailID = types.StringValue(n.Email.EmailID)
	} else {
		plan.EmailID = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *emailNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state emailNotificationModel
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
		resp.Diagnostics.AddError("Error reading email notification", err.Error())
		return
	}
	state.Name = types.StringValue(n.Name)
	if n.EmailID != nil {
		state.EmailID = types.StringValue(*n.EmailID)
	} else if n.Email != nil {
		state.EmailID = types.StringValue(n.Email.EmailID)
	}
	if n.Email != nil {
		state.SMTPServer = types.StringValue(n.Email.SMTPServer)
		state.SMTPPort = types.Int64Value(int64(n.Email.SMTPPort))
		state.Username = types.StringValue(n.Email.Username)
		if n.Email.Password != "" {
			state.Password = types.StringValue(n.Email.Password)
		}
		state.FromAddress = types.StringValue(n.Email.FromAddress)
		if len(n.Email.ToAddresses) > 0 {
			listVal, diags := types.ListValueFrom(ctx, types.StringType, n.Email.ToAddresses)
			resp.Diagnostics.Append(diags...)
			if !diags.HasError() {
				state.ToAddresses = listVal
			}
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

func (r *emailNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan emailNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state emailNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, err := plan.toInput(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading plan", err.Error())
		return
	}
	emailID := state.EmailID.ValueString()
	if err := r.client.UpdateEmailNotification(ctx, plan.ID.ValueString(), emailID, in); err != nil {
		resp.Diagnostics.AddError("Error updating email notification", err.Error())
		return
	}
	plan.EmailID = state.EmailID
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *emailNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state emailNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNotification(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting email notification", err.Error())
	}
}

func (r *emailNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
