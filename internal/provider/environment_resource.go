package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ resource.Resource                = &environmentResource{}
	_ resource.ResourceWithConfigure   = &environmentResource{}
	_ resource.ResourceWithImportState = &environmentResource{}
)

type environmentResource struct {
	client *client.Client
}

func NewEnvironmentResource() resource.Resource {
	return &environmentResource{}
}

type environmentModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Env         types.Map    `tfsdk:"env"`
}

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A custom (non-production) environment inside a Dokploy project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project the environment belongs to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Environment name, e.g. `staging`. Do not use `production` (managed via dokploy_project).",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Environment description.",
			},
			"env": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Shared environment variables for all applications in this environment.",
			},
		},
	}
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API's environment.create endpoint does not accept an env field;
	// environment variables must be set via environment.update after creation.
	env, err := r.client.CreateEnvironment(ctx, client.EnvironmentInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		ProjectID:   plan.ProjectID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating environment", err.Error())
		return
	}

	plan.ID = types.StringValue(env.ID)

	// Set env variables via update if specified.
	if !plan.Env.IsNull() {
		envStr, convErr := envMapToString(ctx, plan.Env)
		if convErr != nil {
			resp.Diagnostics.AddError("Error reading env", convErr.Error())
			return
		}
		if envStr != "" {
			if _, updateErr := r.client.UpdateEnvironment(ctx, env.ID, client.EnvironmentInput{Env: envStr}); updateErr != nil {
				resp.Diagnostics.AddError("Error setting env", updateErr.Error())
				return
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.GetEnvironment(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading environment", err.Error())
		return
	}

	state.Name = types.StringValue(env.Name)
	state.ProjectID = types.StringValue(env.ProjectID)
	if env.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(env.Description)
	}
	// Populate env from API. When the API returns a non-empty env string, parse
	// it into the map. When it is empty and the configured state has env set,
	// use an empty map (preserves intent). When env was never configured (null
	// state) and the API returns empty, keep it null so there is no spurious diff.
	if env.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(env.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envStr, convErr := envMapToString(ctx, plan.Env)
	if convErr != nil {
		resp.Diagnostics.AddError("Error reading env", convErr.Error())
		return
	}

	if _, err := r.client.UpdateEnvironment(ctx, plan.ID.ValueString(), client.EnvironmentInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Env:         envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating environment", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteEnvironment(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting environment", err.Error())
	}
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
