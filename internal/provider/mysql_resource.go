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
	_ resource.Resource                = &mysqlResource{}
	_ resource.ResourceWithConfigure   = &mysqlResource{}
	_ resource.ResourceWithImportState = &mysqlResource{}
)

type mysqlResource struct{ client *client.Client }

func NewMysqlResource() resource.Resource { return &mysqlResource{} }

type mysqlModel struct {
	ID                   types.String   `tfsdk:"id"`
	EnvironmentID        types.String   `tfsdk:"environment_id"`
	Name                 types.String   `tfsdk:"name"`
	Description          types.String   `tfsdk:"description"`
	DockerImage          types.String   `tfsdk:"docker_image"`
	ExternalPort         types.Int64    `tfsdk:"external_port"`
	Env                  types.Map      `tfsdk:"env"`
	DatabaseName         types.String   `tfsdk:"database_name"`
	DatabaseUser         types.String   `tfsdk:"database_user"`
	DatabasePassword     types.String   `tfsdk:"database_password"`
	DatabaseRootPassword types.String   `tfsdk:"database_root_password"`
	AppName              types.String   `tfsdk:"app_name"`
	Status               types.String   `tfsdk:"status"`
	Timeouts             timeouts.Value `tfsdk:"timeouts"`
}

func (r *mysqlResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mysql"
}

func (r *mysqlResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy-managed MySQL database service. Deployed and polled on apply.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"environment_id": schema.StringAttribute{Required: true, MarkdownDescription: "Environment the database belongs to. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description":    schema.StringAttribute{Optional: true, MarkdownDescription: "Description. Once set, removing this attribute does not clear it on the server."},
			"docker_image":   schema.StringAttribute{Required: true, MarkdownDescription: "MySQL image, e.g. `mysql:8`."},
			"external_port":  schema.Int64Attribute{Optional: true, MarkdownDescription: "Host port to expose."},
			"env":            schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Extra environment variables."},
			"database_name":  schema.StringAttribute{Required: true, MarkdownDescription: "Initial database name."},
			"database_user":  schema.StringAttribute{Required: true, MarkdownDescription: "Database user."},
			"database_password": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				MarkdownDescription: "Database password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"database_root_password": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				MarkdownDescription: "MySQL root password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Internal service name (Dokploy-generated).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"status":   schema.StringAttribute{Computed: true, MarkdownDescription: "Status of the most recent deploy."},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *mysqlResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *mysqlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mysqlModel
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
	rootPassword := resolvePassword(plan.DatabaseRootPassword)
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}

	my, err := r.client.CreateMysql(ctx, client.MysqlInput{
		Name:                 plan.Name.ValueString(),
		AppName:              slugify(plan.Name.ValueString()),
		Description:          plan.Description.ValueString(),
		EnvironmentID:        plan.EnvironmentID.ValueString(),
		DockerImage:          plan.DockerImage.ValueString(),
		DatabaseName:         plan.DatabaseName.ValueString(),
		DatabaseUser:         plan.DatabaseUser.ValueString(),
		DatabasePassword:     password,
		DatabaseRootPassword: rootPassword,
		ExternalPort:         int(plan.ExternalPort.ValueInt64()),
		Env:                  envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating mysql", err.Error())
		return
	}
	plan.ID = types.StringValue(my.ID)
	plan.AppName = types.StringValue(my.AppName)
	plan.DatabasePassword = types.StringValue(password)
	plan.DatabaseRootPassword = types.StringValue(rootPassword)

	deployFn := func(ctx context.Context) error { return r.client.DeployMysql(ctx, my.ID) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMysql(ctx, my.ID)
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("MySQL deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mysqlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mysqlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	my, err := r.client.GetMysql(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading mysql", err.Error())
		return
	}
	state.Name = types.StringValue(my.Name)
	state.EnvironmentID = types.StringValue(my.EnvironmentID)
	state.DockerImage = types.StringValue(my.DockerImage)
	state.AppName = types.StringValue(my.AppName)
	state.Status = types.StringValue(my.ApplicationStatus)
	state.DatabaseName = types.StringValue(my.DatabaseName)
	state.DatabaseUser = types.StringValue(my.DatabaseUser)
	if my.DatabasePassword != "" {
		state.DatabasePassword = types.StringValue(my.DatabasePassword)
	}
	if my.DatabaseRootPassword != "" {
		state.DatabaseRootPassword = types.StringValue(my.DatabaseRootPassword)
	}
	if my.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(my.Description)
	}
	if my.ExternalPort != 0 || !state.ExternalPort.IsNull() {
		state.ExternalPort = types.Int64Value(int64(my.ExternalPort))
	}
	if my.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(my.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *mysqlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mysqlModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state mysqlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDatabaseTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	password := plan.DatabasePassword.ValueString()
	if password == "" {
		password = state.DatabasePassword.ValueString()
	}
	rootPassword := plan.DatabaseRootPassword.ValueString()
	if rootPassword == "" {
		rootPassword = state.DatabaseRootPassword.ValueString()
	}
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	if err := r.client.UpdateMysql(ctx, plan.ID.ValueString(), client.MysqlInput{
		Name:                 plan.Name.ValueString(),
		Description:          plan.Description.ValueString(),
		DockerImage:          plan.DockerImage.ValueString(),
		DatabaseName:         plan.DatabaseName.ValueString(),
		DatabaseUser:         plan.DatabaseUser.ValueString(),
		DatabasePassword:     password,
		DatabaseRootPassword: rootPassword,
		ExternalPort:         int(plan.ExternalPort.ValueInt64()),
		Env:                  envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating mysql", err.Error())
		return
	}
	plan.DatabasePassword = types.StringValue(password)
	plan.DatabaseRootPassword = types.StringValue(rootPassword)

	deployFn := func(ctx context.Context) error { return r.client.DeployMysql(ctx, plan.ID.ValueString()) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMysql(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("MySQL deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mysqlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mysqlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMysql(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting mysql", err.Error())
	}
}

func (r *mysqlResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
