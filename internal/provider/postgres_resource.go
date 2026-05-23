package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
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
	_ resource.Resource                = &postgresResource{}
	_ resource.ResourceWithConfigure   = &postgresResource{}
	_ resource.ResourceWithImportState = &postgresResource{}
)

type postgresResource struct {
	client *client.Client
}

func NewPostgresResource() resource.Resource { return &postgresResource{} }

type postgresModel struct {
	ID               types.String   `tfsdk:"id"`
	EnvironmentID    types.String   `tfsdk:"environment_id"`
	Name             types.String   `tfsdk:"name"`
	Description      types.String   `tfsdk:"description"`
	DockerImage      types.String   `tfsdk:"docker_image"`
	ExternalPort     types.Int64    `tfsdk:"external_port"`
	Env              types.Map      `tfsdk:"env"`
	DatabaseName     types.String   `tfsdk:"database_name"`
	DatabaseUser     types.String   `tfsdk:"database_user"`
	DatabasePassword types.String   `tfsdk:"database_password"`
	AppName          types.String   `tfsdk:"app_name"`
	Status           types.String   `tfsdk:"status"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *postgresResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_postgres"
}

func (r *postgresResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy-managed PostgreSQL database service. Deployed and polled on apply.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Environment the database belongs to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description. Note: once set, removing this attribute does not clear it on the server.",
			},
			"docker_image": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "PostgreSQL image, e.g. `postgres:16`.",
			},
			"external_port": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Host port to expose the database on. Omit to keep internal-only.",
			},
			"env": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Extra environment variables.",
			},
			"database_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Initial database name.",
			},
			"database_user": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Database user.",
			},
			"database_password": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Database password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state. Changing this triggers a re-deploy, but only affects fresh containers — see the docs.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal service name (Dokploy-generated). Use this as the hostname from other services inside Dokploy's network.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of the most recent deploy.",
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *postgresResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *postgresResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan postgresModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	password := resolvePassword(plan.DatabasePassword)
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}

	pg, err := r.client.CreatePostgres(ctx, client.PostgresInput{
		Name:             plan.Name.ValueString(),
		AppName:          slugify(plan.Name.ValueString()),
		Description:      plan.Description.ValueString(),
		EnvironmentID:    plan.EnvironmentID.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabaseName:     plan.DatabaseName.ValueString(),
		DatabaseUser:     plan.DatabaseUser.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating postgres", err.Error())
		return
	}

	plan.ID = types.StringValue(pg.ID)
	plan.AppName = types.StringValue(pg.AppName)
	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error {
		return r.client.DeployPostgres(ctx, pg.ID)
	}
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetPostgres(ctx, pg.ID)
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Postgres deploy failed", err.Error())
		return
	}

	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *postgresResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state postgresModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pg, err := r.client.GetPostgres(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading postgres", err.Error())
		return
	}

	state.Name = types.StringValue(pg.Name)
	state.EnvironmentID = types.StringValue(pg.EnvironmentID)
	state.DockerImage = types.StringValue(pg.DockerImage)
	state.AppName = types.StringValue(pg.AppName)
	state.Status = types.StringValue(pg.ApplicationStatus)
	state.DatabaseName = types.StringValue(pg.DatabaseName)
	state.DatabaseUser = types.StringValue(pg.DatabaseUser)
	if pg.DatabasePassword != "" {
		state.DatabasePassword = types.StringValue(pg.DatabasePassword)
	}
	if pg.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(pg.Description)
	}
	if pg.ExternalPort != 0 || !state.ExternalPort.IsNull() {
		state.ExternalPort = types.Int64Value(int64(pg.ExternalPort))
	}
	if pg.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(pg.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *postgresResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan postgresModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state postgresModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve previously generated password if the user removed the attribute.
	password := plan.DatabasePassword.ValueString()
	if password == "" {
		password = state.DatabasePassword.ValueString()
	}
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}

	if err := r.client.UpdatePostgres(ctx, plan.ID.ValueString(), client.PostgresInput{
		Name:             plan.Name.ValueString(),
		Description:      plan.Description.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabaseName:     plan.DatabaseName.ValueString(),
		DatabaseUser:     plan.DatabaseUser.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating postgres", err.Error())
		return
	}

	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error {
		return r.client.DeployPostgres(ctx, plan.ID.ValueString())
	}
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetPostgres(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Postgres deploy failed", err.Error())
		return
	}

	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *postgresResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state postgresModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePostgres(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting postgres", err.Error())
	}
}

func (r *postgresResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
