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
	_ resource.Resource                = &destinationResource{}
	_ resource.ResourceWithConfigure   = &destinationResource{}
	_ resource.ResourceWithImportState = &destinationResource{}
)

type destinationResource struct {
	client *client.Client
}

func NewDestinationResource() resource.Resource { return &destinationResource{} }

type destinationModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	ProviderType    types.String `tfsdk:"provider_type"`
	Bucket          types.String `tfsdk:"bucket"`
	Endpoint        types.String `tfsdk:"endpoint"`
	Region          types.String `tfsdk:"region"`
	AccessKey       types.String `tfsdk:"access_key"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	AdditionalFlags types.List   `tfsdk:"additional_flags"`
	OrganizationID  types.String `tfsdk:"organization_id"`
}

func (r *destinationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_destination"
}

func (r *destinationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An S3-compatible storage destination used by `dokploy_backup`. Lives at the organization level.",
		Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":              schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"provider_type":     schema.StringAttribute{Required: true, MarkdownDescription: "S3 provider (free-form string; observed values: `DigitalOcean`, `AWS`). Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"bucket":            schema.StringAttribute{Required: true, MarkdownDescription: "S3 bucket name."},
			"endpoint":          schema.StringAttribute{Required: true, MarkdownDescription: "S3 endpoint URL."},
			"region":            schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "S3 region (empty for DO Spaces, required for AWS).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"access_key":        schema.StringAttribute{Required: true, MarkdownDescription: "S3 access key id."},
			"secret_access_key": schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "S3 secret access key."},
			"additional_flags": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Extra flags forwarded to the backup tool (rclone-style).",
			},
			"organization_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization the destination belongs to (set by the API key).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *destinationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// stringListToGo converts a types.List of strings to []string.
func stringListToGo(ctx context.Context, l types.List) ([]string, error) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	out := []string{}
	if diags := l.ElementsAs(ctx, &out, false); diags.HasError() {
		return nil, fmt.Errorf("converting string list")
	}
	return out, nil
}

func (r *destinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan destinationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flags, err := stringListToGo(ctx, plan.AdditionalFlags)
	if err != nil {
		resp.Diagnostics.AddError("Error reading additional_flags", err.Error())
		return
	}

	d, err := r.client.CreateDestination(ctx, client.DestinationInput{
		Name:            plan.Name.ValueString(),
		Provider:        plan.ProviderType.ValueString(),
		Bucket:          plan.Bucket.ValueString(),
		Endpoint:        plan.Endpoint.ValueString(),
		Region:          plan.Region.ValueString(),
		AccessKey:       plan.AccessKey.ValueString(),
		SecretAccessKey: plan.SecretAccessKey.ValueString(),
		AdditionalFlags: flags,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating destination", err.Error())
		return
	}

	plan.ID = types.StringValue(d.ID)
	plan.OrganizationID = types.StringValue(d.OrganizationID)
	if plan.Region.IsNull() || plan.Region.IsUnknown() {
		plan.Region = types.StringValue(d.Region)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *destinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state destinationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.client.GetDestination(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading destination", err.Error())
		return
	}
	state.Name = types.StringValue(d.Name)
	state.ProviderType = types.StringValue(d.Provider)
	state.Bucket = types.StringValue(d.Bucket)
	state.Endpoint = types.StringValue(d.Endpoint)
	state.Region = types.StringValue(d.Region)
	state.AccessKey = types.StringValue(d.AccessKey)
	state.SecretAccessKey = types.StringValue(d.SecretAccessKey)
	state.OrganizationID = types.StringValue(d.OrganizationID)
	if len(d.AdditionalFlags) > 0 || !state.AdditionalFlags.IsNull() {
		flagsList, diags := types.ListValueFrom(ctx, types.StringType, d.AdditionalFlags)
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.AdditionalFlags = flagsList
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *destinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan destinationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flags, err := stringListToGo(ctx, plan.AdditionalFlags)
	if err != nil {
		resp.Diagnostics.AddError("Error reading additional_flags", err.Error())
		return
	}

	if err := r.client.UpdateDestination(ctx, plan.ID.ValueString(), client.DestinationInput{
		Name:            plan.Name.ValueString(),
		Provider:        plan.ProviderType.ValueString(),
		Bucket:          plan.Bucket.ValueString(),
		Endpoint:        plan.Endpoint.ValueString(),
		Region:          plan.Region.ValueString(),
		AccessKey:       plan.AccessKey.ValueString(),
		SecretAccessKey: plan.SecretAccessKey.ValueString(),
		AdditionalFlags: flags,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating destination", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *destinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state destinationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDestination(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting destination", err.Error())
	}
}

func (r *destinationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
