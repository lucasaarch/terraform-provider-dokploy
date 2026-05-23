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
	_ resource.Resource                = &redisResource{}
	_ resource.ResourceWithConfigure   = &redisResource{}
	_ resource.ResourceWithImportState = &redisResource{}
)

type redisResource struct{ client *client.Client }

func NewRedisResource() resource.Resource { return &redisResource{} }

type redisModel struct {
	ID               types.String   `tfsdk:"id"`
	EnvironmentID    types.String   `tfsdk:"environment_id"`
	Name             types.String   `tfsdk:"name"`
	Description      types.String   `tfsdk:"description"`
	DockerImage      types.String   `tfsdk:"docker_image"`
	ExternalPort     types.Int64    `tfsdk:"external_port"`
	Env              types.Map      `tfsdk:"env"`
	DatabasePassword types.String   `tfsdk:"database_password"`
	AppName          types.String   `tfsdk:"app_name"`
	Status           types.String   `tfsdk:"status"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *redisResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_redis"
}

func (r *redisResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy-managed Redis service. Deployed and polled on apply.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"environment_id": schema.StringAttribute{Required: true, MarkdownDescription: "Environment the cache belongs to. Changing this forces replacement.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description":    schema.StringAttribute{Optional: true, MarkdownDescription: "Description. Once set, removing this attribute does not clear it on the server."},
			"docker_image":   schema.StringAttribute{Required: true, MarkdownDescription: "Redis image, e.g. `redis:7.2`."},
			"external_port":  schema.Int64Attribute{Optional: true, MarkdownDescription: "Host port to expose."},
			"env":            schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Extra environment variables."},
			"database_password": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				MarkdownDescription: "Redis `requirepass` value. If omitted, the provider generates a 32-character `[a-zA-Z0-9]` password and stores it in state.",
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

func (r *redisResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *redisResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan redisModel
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
	re, err := r.client.CreateRedis(ctx, client.RedisInput{
		Name:             plan.Name.ValueString(),
		AppName:          slugify(plan.Name.ValueString()),
		Description:      plan.Description.ValueString(),
		EnvironmentID:    plan.EnvironmentID.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating redis", err.Error())
		return
	}
	plan.ID = types.StringValue(re.ID)
	plan.AppName = types.StringValue(re.AppName)
	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error { return r.client.DeployRedis(ctx, re.ID) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetRedis(ctx, re.ID)
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, createTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Redis deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *redisResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state redisModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	re, err := r.client.GetRedis(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading redis", err.Error())
		return
	}
	state.Name = types.StringValue(re.Name)
	state.EnvironmentID = types.StringValue(re.EnvironmentID)
	state.DockerImage = types.StringValue(re.DockerImage)
	state.AppName = types.StringValue(re.AppName)
	state.Status = types.StringValue(re.ApplicationStatus)
	if re.DatabasePassword != "" {
		state.DatabasePassword = types.StringValue(re.DatabasePassword)
	}
	if re.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(re.Description)
	}
	if re.ExternalPort != 0 || !state.ExternalPort.IsNull() {
		state.ExternalPort = types.Int64Value(int64(re.ExternalPort))
	}
	if re.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(re.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *redisResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan redisModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state redisModel
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
	envStr, err := envMapToString(ctx, plan.Env)
	if err != nil {
		resp.Diagnostics.AddError("Error reading env", err.Error())
		return
	}
	if err := r.client.UpdateRedis(ctx, plan.ID.ValueString(), client.RedisInput{
		Name:             plan.Name.ValueString(),
		Description:      plan.Description.ValueString(),
		DockerImage:      plan.DockerImage.ValueString(),
		DatabasePassword: password,
		ExternalPort:     int(plan.ExternalPort.ValueInt64()),
		Env:              envStr,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating redis", err.Error())
		return
	}
	plan.DatabasePassword = types.StringValue(password)

	deployFn := func(ctx context.Context) error { return r.client.DeployRedis(ctx, plan.ID.ValueString()) }
	statusFn := func(ctx context.Context) (string, error) {
		got, err := r.client.GetRedis(ctx, plan.ID.ValueString())
		if err != nil {
			return "", err
		}
		return got.ApplicationStatus, nil
	}
	if err := deployAndWait(ctx, deployFn, statusFn, databasePollInterval, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Redis deploy failed", err.Error())
		return
	}
	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *redisResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state redisModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteRedis(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting redis", err.Error())
	}
}

func (r *redisResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
