package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &serverResource{}
	_ resource.ResourceWithConfigure   = &serverResource{}
	_ resource.ResourceWithImportState = &serverResource{}
)

type serverResource struct {
	client *client.Client
}

func NewServerResource() resource.Resource { return &serverResource{} }

type serverModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	IPAddress      types.String `tfsdk:"ip_address"`
	Port           types.Int64  `tfsdk:"port"`
	Username       types.String `tfsdk:"username"`
	SshKeyID       types.String `tfsdk:"ssh_key_id"`
	ServerType     types.String `tfsdk:"server_type"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *serverResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (r *serverResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A remote machine registered as a managed worker in Dokploy. Creating this resource performs an SSH handshake — the corresponding public key (from `dokploy_ssh_key.x.public_key`) must already be in the remote's `~/.ssh/authorized_keys`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":            schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Description (required by the API as a present field; defaults to empty string)."},
			"ip_address":      schema.StringAttribute{Required: true, MarkdownDescription: "IP address or hostname. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"port":            schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(22), MarkdownDescription: "SSH port (default 22)."},
			"username":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("root"), MarkdownDescription: "SSH username (default `root`)."},
			"ssh_key_id":      schema.StringAttribute{Required: true, MarkdownDescription: "`dokploy_ssh_key.x.id`. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"server_type":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("deploy"), MarkdownDescription: "`deploy` (default — runs workloads) or `build` (used as a build host)."},
			"organization_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization the server belongs to (derived from the SSH key).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *serverResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m serverModel) toInput() client.ServerInput {
	return client.ServerInput{
		Name:        m.Name.ValueString(),
		Description: m.Description.ValueString(),
		IPAddress:   m.IPAddress.ValueString(),
		Port:        int(m.Port.ValueInt64()),
		Username:    m.Username.ValueString(),
		SshKeyID:    m.SshKeyID.ValueString(),
		ServerType:  m.ServerType.ValueString(),
	}
}

func (r *serverResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.CreateServer(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating server (SSH handshake may have failed — verify the public key is in the remote authorized_keys)", err.Error())
		return
	}
	plan.ID = types.StringValue(s.ID)
	plan.OrganizationID = types.StringValue(s.OrganizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serverResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.GetServer(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading server", err.Error())
		return
	}
	state.Name = types.StringValue(s.Name)
	state.Description = types.StringValue(s.Description)
	state.IPAddress = types.StringValue(s.IPAddress)
	state.Port = types.Int64Value(int64(s.Port))
	state.Username = types.StringValue(s.Username)
	state.SshKeyID = types.StringValue(s.SshKeyID)
	state.ServerType = types.StringValue(s.ServerType)
	state.OrganizationID = types.StringValue(s.OrganizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *serverResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdateServer(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating server", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serverResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteServer(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting server", err.Error())
	}
}

func (r *serverResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
