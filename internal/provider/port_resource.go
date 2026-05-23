package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &portResource{}
	_ resource.ResourceWithConfigure   = &portResource{}
	_ resource.ResourceWithImportState = &portResource{}
)

type portResource struct{ client *client.Client }

func NewPortResource() resource.Resource { return &portResource{} }

type portModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	PublishedPort types.Int64  `tfsdk:"published_port"`
	TargetPort    types.Int64  `tfsdk:"target_port"`
	Protocol      types.String `tfsdk:"protocol"`
}

func (r *portResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port"
}

func (r *portResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A published port on a Dokploy application (host port → container port).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"application_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application that owns the port. Changing forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"published_port": schema.Int64Attribute{Required: true, MarkdownDescription: "Host port (published)."},
			"target_port":    schema.Int64Attribute{Required: true, MarkdownDescription: "Container port (target)."},
			"protocol": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("tcp"),
				MarkdownDescription: "`tcp` (default) or `udp`.",
			},
		},
	}
}

func (r *portResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m portModel) toInput() client.PortInput {
	return client.PortInput{
		ApplicationID: m.ApplicationID.ValueString(),
		PublishedPort: int(m.PublishedPort.ValueInt64()),
		TargetPort:    int(m.TargetPort.ValueInt64()),
		Protocol:      m.Protocol.ValueString(),
	}
}

func (r *portResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan portModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.CreatePort(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating port", err.Error())
		return
	}
	plan.ID = types.StringValue(p.ID)
	if p.Protocol != "" {
		plan.Protocol = types.StringValue(p.Protocol)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *portResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state portModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.GetPort(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading port", err.Error())
		return
	}
	state.ApplicationID = types.StringValue(p.ApplicationID)
	state.PublishedPort = types.Int64Value(int64(p.PublishedPort))
	state.TargetPort = types.Int64Value(int64(p.TargetPort))
	state.Protocol = types.StringValue(p.Protocol)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *portResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan portModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdatePort(ctx, plan.ID.ValueString(), plan.toInput()); err != nil {
		resp.Diagnostics.AddError("Error updating port", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *portResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state portModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePort(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting port", err.Error())
	}
}

func (r *portResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
