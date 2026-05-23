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
	_ resource.Resource                = &mariadbResource{}
	_ resource.ResourceWithConfigure   = &mariadbResource{}
	_ resource.ResourceWithImportState = &mariadbResource{}
)

type mariadbResource struct{ client *client.Client }

func NewMariadbResource() resource.Resource { return &mariadbResource{} }

type mariadbModel struct {
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
	ServerID             types.String   `tfsdk:"server_id"`
	Timeouts             timeouts.Value `tfsdk:"timeouts"`
}

func (r *mariadbResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mariadb"
}

func (r *mariadbResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy-managed MariaDB database service. Deployed and polled on apply.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"environment_id": schema.StringAttribute{Required: true, MarkdownDescription: "Environment the database belongs to. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description":    schema.StringAttribute{Optional: true, MarkdownDescription: "Description. Once set, removing this attribute does not clear it on the server."},
			"docker_image":   schema.StringAttribute{Required: true, MarkdownDescription: "MariaDB image, e.g. `mariadb:11`."},
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
				MarkdownDescription: "MariaDB root password. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Internal service name (Dokploy-generated).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"server_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Managed server (`dokploy_server.x.id`) the database runs on. Omit to run on the Dokploy host. Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{Computed: true, MarkdownDescription: "Status of the most recent deploy."},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *mariadbResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *mariadbResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mariadbModel
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
	ma, err := r.client.CreateMariadb(ctx, client.MariadbInput{
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
		ServerID:             optionalString(plan.ServerID),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating mariadb", err.Error())
		return
	}
	plan.ID = types.StringValue(ma.ID)
	plan.AppName = types.StringValue(ma.AppName)
	plan.DatabasePassword = types.StringValue(password)
	plan.DatabaseRootPassword = types.StringValue(rootPassword)
	if ma.ServerID != nil {
		plan.ServerID = types.StringValue(*ma.ServerID)
	} else if plan.ServerID.IsUnknown() {
		plan.ServerID = types.StringNull()
	}

	deployFn := func(ctx context.Context) error { return r.client.DeployMariadb(ctx, ma.ID) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMariadb(ctx, ma.ID)
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("MariaDB deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mariadbResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mariadbModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ma, err := r.client.GetMariadb(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading mariadb", err.Error())
		return
	}
	state.Name = types.StringValue(ma.Name)
	state.EnvironmentID = types.StringValue(ma.EnvironmentID)
	state.DockerImage = types.StringValue(ma.DockerImage)
	state.AppName = types.StringValue(ma.AppName)
	state.Status = types.StringValue(ma.ApplicationStatus)
	state.DatabaseName = types.StringValue(ma.DatabaseName)
	state.DatabaseUser = types.StringValue(ma.DatabaseUser)
	if ma.DatabasePassword != "" {
		state.DatabasePassword = types.StringValue(ma.DatabasePassword)
	}
	if ma.DatabaseRootPassword != "" {
		state.DatabaseRootPassword = types.StringValue(ma.DatabaseRootPassword)
	}
	if ma.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(ma.Description)
	}
	if ma.ExternalPort != 0 || !state.ExternalPort.IsNull() {
		state.ExternalPort = types.Int64Value(int64(ma.ExternalPort))
	}
	if ma.ServerID != nil {
		state.ServerID = types.StringValue(*ma.ServerID)
	} else {
		state.ServerID = types.StringNull()
	}
	if ma.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(ma.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *mariadbResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mariadbModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state mariadbModel
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
	if err := r.client.UpdateMariadb(ctx, plan.ID.ValueString(), client.MariadbInput{
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
		resp.Diagnostics.AddError("Error updating mariadb", err.Error())
		return
	}
	plan.DatabasePassword = types.StringValue(password)
	plan.DatabaseRootPassword = types.StringValue(rootPassword)

	deployFn := func(ctx context.Context) error { return r.client.DeployMariadb(ctx, plan.ID.ValueString()) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetMariadb(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("MariaDB deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *mariadbResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mariadbModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMariadb(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting mariadb", err.Error())
	}
}

func (r *mariadbResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
