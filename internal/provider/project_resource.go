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
	_ resource.Resource                = &projectResource{}
	_ resource.ResourceWithConfigure   = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
)

type projectResource struct {
	client *client.Client
}

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

type projectModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	OrganizationID          types.String `tfsdk:"organization_id"`
	ProductionEnv           types.Map    `tfsdk:"production_env"`
	ProductionEnvironmentID types.String `tfsdk:"production_environment_id"`
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy project. Each project owns an auto-created `production` environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project description.",
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization the project belongs to. Determined by the API key; not configurable.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"production_env": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Shared environment variables for the auto-created `production` environment.",
			},
			"production_environment_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier of the auto-created `production` environment.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

// envMapToString converts a types.Map into a dotenv string.
func envMapToString(ctx context.Context, m types.Map) (string, error) {
	if m.IsNull() || m.IsUnknown() {
		return "", nil
	}
	raw := map[string]string{}
	if diags := m.ElementsAs(ctx, &raw, false); diags.HasError() {
		return "", fmt.Errorf("converting env map")
	}
	return client.MapToDotenv(raw), nil
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	proj, err := r.client.CreateProject(ctx, client.ProjectInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	prodEnvID := proj.ProductionEnvironmentID()

	// Apply production_env to the auto-created production environment.
	if !plan.ProductionEnv.IsNull() {
		envStr, convErr := envMapToString(ctx, plan.ProductionEnv)
		if convErr != nil {
			resp.Diagnostics.AddError("Error reading production_env", convErr.Error())
			return
		}
		if _, err := r.client.UpdateEnvironment(ctx, prodEnvID, client.EnvironmentInput{Env: envStr}); err != nil {
			resp.Diagnostics.AddError("Error setting production_env", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(proj.ID)
	plan.OrganizationID = types.StringValue(proj.OrganizationID)
	plan.ProductionEnvironmentID = types.StringValue(prodEnvID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	proj, err := r.client.GetProject(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	state.Name = types.StringValue(proj.Name)
	if proj.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(proj.Description)
	}
	state.OrganizationID = types.StringValue(proj.OrganizationID)
	state.ProductionEnvironmentID = types.StringValue(proj.ProductionEnvironmentID())
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.UpdateProject(ctx, plan.ID.ValueString(), client.ProjectInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}

	envStr, convErr := envMapToString(ctx, plan.ProductionEnv)
	if convErr != nil {
		resp.Diagnostics.AddError("Error reading production_env", convErr.Error())
		return
	}
	if _, err := r.client.UpdateEnvironment(ctx, plan.ProductionEnvironmentID.ValueString(),
		client.EnvironmentInput{Env: envStr}); err != nil {
		resp.Diagnostics.AddError("Error updating production_env", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteProject(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
