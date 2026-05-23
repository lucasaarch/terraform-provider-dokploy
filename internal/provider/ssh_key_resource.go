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
	_ resource.Resource                = &sshKeyResource{}
	_ resource.ResourceWithConfigure   = &sshKeyResource{}
	_ resource.ResourceWithImportState = &sshKeyResource{}
)

type sshKeyResource struct {
	client *client.Client
}

func NewSshKeyResource() resource.Resource { return &sshKeyResource{} }

type sshKeyModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	PublicKey      types.String `tfsdk:"public_key"`
	PrivateKey     types.String `tfsdk:"private_key"`
}

func (r *sshKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (r *sshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An SSH key registered at the Dokploy organization level. Used by `dokploy_server` to authenticate against the remote machine.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organization that owns the SSH key. Reference `data.dokploy_organization.x.id`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name.",
			},
			"public_key": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Public key in OpenSSH format (`ssh-rsa AAAA...`). If omitted, the provider generates a 4096-bit RSA pair. **Immutable**: changing this value forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"private_key": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Private key in PEM PKCS#1 format. If omitted, the provider generates one. **Immutable**: changing this value forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *sshKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sshKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sshKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priv := plan.PrivateKey.ValueString()
	pub := plan.PublicKey.ValueString()
	if priv == "" || pub == "" {
		generatedPriv, generatedPub, err := generateSSHKeyPair(plan.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error generating SSH key", err.Error())
			return
		}
		if priv == "" {
			priv = generatedPriv
		}
		if pub == "" {
			pub = generatedPub
		}
	}

	k, err := r.client.CreateSshKey(ctx, client.SshKeyInput{
		Name:           plan.Name.ValueString(),
		OrganizationID: plan.OrganizationID.ValueString(),
		PublicKey:      pub,
		PrivateKey:     priv,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating SSH key", err.Error())
		return
	}

	plan.ID = types.StringValue(k.ID)
	plan.PrivateKey = types.StringValue(priv)
	plan.PublicKey = types.StringValue(pub)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *sshKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sshKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	k, err := r.client.GetSshKey(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading SSH key", err.Error())
		return
	}
	state.Name = types.StringValue(k.Name)
	// organization_id: preserve state value; the API may return a different
	// representation (e.g. personal vs org id). It is Required+RequiresReplace
	// so the user controls it and drift would cause spurious replacements.
	if k.OrganizationID != "" && state.OrganizationID.IsNull() {
		state.OrganizationID = types.StringValue(k.OrganizationID)
	}
	if k.PublicKey != "" {
		state.PublicKey = types.StringValue(k.PublicKey)
	}
	if k.PrivateKey != "" {
		state.PrivateKey = types.StringValue(k.PrivateKey)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *sshKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sshKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only name and description are mutable on sshKey.update. Public/private keys
	// are immutable (ForceNew). The schema enforces this at plan time, so by the
	// time Update is reached only name (and maybe description) actually changed.
	if err := r.client.UpdateSshKey(ctx, plan.ID.ValueString(), client.SshKeyUpdateInput{
		Name: plan.Name.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Error updating SSH key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *sshKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sshKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSshKey(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting SSH key", err.Error())
	}
}

func (r *sshKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
