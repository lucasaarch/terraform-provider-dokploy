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
	_ resource.Resource                   = &mountResource{}
	_ resource.ResourceWithConfigure      = &mountResource{}
	_ resource.ResourceWithImportState    = &mountResource{}
	_ resource.ResourceWithValidateConfig = &mountResource{}
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
	case "volume":
		if cfg.VolumeName.IsNull() || cfg.VolumeName.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(path.Root("volume_name"), "volume_name is required when type = \"volume\"", "")
		}
	case "file":
		if cfg.Content.IsNull() || cfg.Content.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(path.Root("content"), "content is required when type = \"file\"", "")
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
