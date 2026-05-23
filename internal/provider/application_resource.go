package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

const (
	defaultDeployTimeout = 10 * time.Minute
	deployPollInterval   = 5 * time.Second
)

var (
	_ resource.Resource                = &applicationResource{}
	_ resource.ResourceWithConfigure   = &applicationResource{}
	_ resource.ResourceWithImportState = &applicationResource{}
)

type applicationResource struct {
	client *client.Client
}

func NewApplicationResource() resource.Resource {
	return &applicationResource{}
}

type applicationModel struct {
	ID               types.String   `tfsdk:"id"`
	EnvironmentID    types.String   `tfsdk:"environment_id"`
	Name             types.String   `tfsdk:"name"`
	Description      types.String   `tfsdk:"description"`
	DockerImage      types.String   `tfsdk:"docker_image"`
	RegistryURL      types.String   `tfsdk:"registry_url"`
	RegistryUsername types.String   `tfsdk:"registry_username"`
	RegistryPassword types.String   `tfsdk:"registry_password"`
	Env              types.Map      `tfsdk:"env"`
	AppName          types.String   `tfsdk:"app_name"`
	Status           types.String   `tfsdk:"status"`
	ServerID         types.String   `tfsdk:"server_id"`
	Replicas         types.Int64    `tfsdk:"replicas"`
	HealthCheck      types.Object   `tfsdk:"health_check"`
	RestartPolicy    types.Object   `tfsdk:"restart_policy"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

// healthCheckAttrTypes are the attribute types for the health_check block object.
var healthCheckAttrTypes = map[string]attr.Type{
	"test":         types.ListType{ElemType: types.StringType},
	"interval":     types.StringType,
	"timeout":      types.StringType,
	"retries":      types.Int64Type,
	"start_period": types.StringType,
}

// restartPolicyAttrTypes are the attribute types for the restart_policy block object.
var restartPolicyAttrTypes = map[string]attr.Type{
	"condition":    types.StringType,
	"delay":        types.StringType,
	"max_attempts": types.Int64Type,
	"window":       types.StringType,
}

// healthCheckFromModel converts a types.Object (health_check block) to *client.HealthCheckSwarm.
func healthCheckFromModel(ctx context.Context, m types.Object) (*client.HealthCheckSwarm, diag.Diagnostics) {
	if m.IsNull() || m.IsUnknown() {
		return nil, nil
	}
	var inner struct {
		Test        types.List   `tfsdk:"test"`
		Interval    types.String `tfsdk:"interval"`
		Timeout     types.String `tfsdk:"timeout"`
		Retries     types.Int64  `tfsdk:"retries"`
		StartPeriod types.String `tfsdk:"start_period"`
	}
	diags := m.As(ctx, &inner, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	var test []string
	if !inner.Test.IsNull() && !inner.Test.IsUnknown() {
		diags.Append(inner.Test.ElementsAs(ctx, &test, false)...)
	}
	hc := &client.HealthCheckSwarm{
		Test:    test,
		Retries: int(inner.Retries.ValueInt64()),
	}
	if s := inner.Interval.ValueString(); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			hc.Interval = d.Nanoseconds()
		}
	}
	if s := inner.Timeout.ValueString(); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			hc.Timeout = d.Nanoseconds()
		}
	}
	if s := inner.StartPeriod.ValueString(); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			hc.StartPeriod = d.Nanoseconds()
		}
	}
	return hc, diags
}

// restartPolicyFromModel converts a types.Object (restart_policy block) to *client.RestartPolicySwarm.
func restartPolicyFromModel(ctx context.Context, m types.Object) (*client.RestartPolicySwarm, diag.Diagnostics) {
	if m.IsNull() || m.IsUnknown() {
		return nil, nil
	}
	var inner struct {
		Condition   types.String `tfsdk:"condition"`
		Delay       types.String `tfsdk:"delay"`
		MaxAttempts types.Int64  `tfsdk:"max_attempts"`
		Window      types.String `tfsdk:"window"`
	}
	diags := m.As(ctx, &inner, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	rp := &client.RestartPolicySwarm{
		Condition:   inner.Condition.ValueString(),
		MaxAttempts: int(inner.MaxAttempts.ValueInt64()),
	}
	if s := inner.Delay.ValueString(); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			rp.Delay = d.Nanoseconds()
		}
	}
	if s := inner.Window.ValueString(); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			rp.Window = d.Nanoseconds()
		}
	}
	return rp, diags
}

// healthCheckToObject converts *client.HealthCheckSwarm to a types.Object for state storage.
func healthCheckToObject(ctx context.Context, hc *client.HealthCheckSwarm) (types.Object, diag.Diagnostics) {
	if hc == nil {
		return types.ObjectNull(healthCheckAttrTypes), nil
	}
	testList, diags := types.ListValueFrom(ctx, types.StringType, hc.Test)
	if diags.HasError() {
		return types.ObjectNull(healthCheckAttrTypes), diags
	}
	return types.ObjectValue(healthCheckAttrTypes, map[string]attr.Value{
		"test":         testList,
		"interval":     types.StringValue(time.Duration(hc.Interval).String()),
		"timeout":      types.StringValue(time.Duration(hc.Timeout).String()),
		"retries":      types.Int64Value(int64(hc.Retries)),
		"start_period": types.StringValue(time.Duration(hc.StartPeriod).String()),
	})
}

// restartPolicyToObject converts *client.RestartPolicySwarm to a types.Object for state storage.
func restartPolicyToObject(rp *client.RestartPolicySwarm) (types.Object, diag.Diagnostics) {
	if rp == nil {
		return types.ObjectNull(restartPolicyAttrTypes), nil
	}
	return types.ObjectValue(restartPolicyAttrTypes, map[string]attr.Value{
		"condition":    types.StringValue(rp.Condition),
		"delay":        types.StringValue(time.Duration(rp.Delay).String()),
		"max_attempts": types.Int64Value(int64(rp.MaxAttempts)),
		"window":       types.StringValue(time.Duration(rp.Window).String()),
	})
}

func (r *applicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (r *applicationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy application sourced from a Docker image. Applying this resource deploys the application and waits for it to finish.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Environment the application belongs to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Application description. Note: once set, removing this attribute from the configuration does not clear the description on the server (the API silently keeps the existing value).",
			},
			"docker_image": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Docker image to deploy, e.g. `nginx:1.27`.",
			},
			"registry_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Private registry URL. Omit for public images.",
			},
			"registry_username": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Registry username.",
			},
			"registry_password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Registry password. Not returned by the API; drift on this field is not detected.",
			},
			"env": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Application environment variables.",
			},
			"app_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal application name generated by Dokploy.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"server_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Managed server (`dokploy_server.x.id`) the application runs on. Omit to run on the Dokploy host. Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of the most recent deployment.",
			},
			"replicas": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Number of Docker Swarm replicas. Defaults to whatever Dokploy uses (typically 1).",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
			"health_check": schema.SingleNestedBlock{
				MarkdownDescription: "Docker Swarm healthcheck. Omit to skip.",
				Attributes: map[string]schema.Attribute{
					"test":         schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Command (e.g. `[\"CMD\", \"curl\", \"-f\", \"http://localhost/health\"]`)."},
					"interval":     schema.StringAttribute{Optional: true, MarkdownDescription: "Probe interval (e.g. `30s`)."},
					"timeout":      schema.StringAttribute{Optional: true, MarkdownDescription: "Probe timeout."},
					"retries":      schema.Int64Attribute{Optional: true, MarkdownDescription: "Number of retries before unhealthy."},
					"start_period": schema.StringAttribute{Optional: true, MarkdownDescription: "Initial grace period."},
				},
			},
			"restart_policy": schema.SingleNestedBlock{
				MarkdownDescription: "Docker Swarm restart policy. Omit to skip.",
				Attributes: map[string]schema.Attribute{
					"condition":    schema.StringAttribute{Optional: true, MarkdownDescription: "`none`, `on-failure`, or `any`."},
					"delay":        schema.StringAttribute{Optional: true, MarkdownDescription: "Delay between attempts (e.g. `5s`)."},
					"max_attempts": schema.Int64Attribute{Optional: true, MarkdownDescription: "Maximum restart attempts."},
					"window":       schema.StringAttribute{Optional: true, MarkdownDescription: "Evaluation window for max attempts."},
				},
			},
		},
	}
}

func (r *applicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// optionalString returns a pointer to the string value, or nil when the
// attribute is null/unknown/empty — used for API fields that must serialize as
// JSON null when unset.
func optionalString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil
	}
	s := v.ValueString()
	return &s
}

// configureAndDeploy applies docker provider config + env + swarm settings, triggers a deploy,
// and waits for it to finish. Used by both Create and Update.
func (r *applicationResource) configureAndDeploy(ctx context.Context, m *applicationModel, timeout time.Duration) error {
	id := m.ID.ValueString()

	if err := r.client.SaveDockerProvider(ctx, client.DockerProviderInput{
		ApplicationID: id,
		DockerImage:   m.DockerImage.ValueString(),
		RegistryURL:   m.RegistryURL.ValueString(),
		Username:      optionalString(m.RegistryUsername),
		Password:      optionalString(m.RegistryPassword),
	}); err != nil {
		return fmt.Errorf("saving docker provider: %w", err)
	}

	envStr, err := envMapToString(ctx, m.Env)
	if err != nil {
		return err
	}
	if err := r.client.SaveEnvironment(ctx, id, envStr); err != nil {
		return fmt.Errorf("saving environment: %w", err)
	}

	if err := r.client.Deploy(ctx, id); err != nil {
		return fmt.Errorf("triggering deploy: %w", err)
	}

	deployCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return r.client.WaitForDeployment(deployCtx, id, deployPollInterval)
}

func (r *applicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan applicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultDeployTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.CreateApplication(ctx, client.ApplicationInput{
		Name:          plan.Name.ValueString(),
		AppName:       slugify(plan.Name.ValueString()),
		Description:   plan.Description.ValueString(),
		EnvironmentID: plan.EnvironmentID.ValueString(),
		ServerID:      optionalString(plan.ServerID),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating application", err.Error())
		return
	}
	plan.ID = types.StringValue(app.ID)
	plan.AppName = types.StringValue(app.AppName)
	if app.ServerID != nil {
		plan.ServerID = types.StringValue(*app.ServerID)
	} else if plan.ServerID.IsUnknown() {
		plan.ServerID = types.StringNull()
	}
	// Initialize replicas from response if not set in plan
	if plan.Replicas.IsUnknown() || plan.Replicas.IsNull() {
		if app.Replicas != nil {
			plan.Replicas = types.Int64Value(int64(*app.Replicas))
		} else {
			plan.Replicas = types.Int64Null()
		}
	}
	// Initialize health_check and restart_policy null objects if not set
	if plan.HealthCheck.IsUnknown() || plan.HealthCheck.IsNull() {
		plan.HealthCheck = types.ObjectNull(healthCheckAttrTypes)
	}
	if plan.RestartPolicy.IsUnknown() || plan.RestartPolicy.IsNull() {
		plan.RestartPolicy = types.ObjectNull(restartPolicyAttrTypes)
	}

	// Apply name/description + swarm advanced fields on the freshly-created application.
	createInput := client.ApplicationInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}
	if !plan.Replicas.IsNull() && !plan.Replicas.IsUnknown() {
		v := int(plan.Replicas.ValueInt64())
		createInput.Replicas = &v
	}
	hcCreate, hcCreateDiags := healthCheckFromModel(ctx, plan.HealthCheck)
	resp.Diagnostics.Append(hcCreateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	createInput.HealthCheckSwarm = hcCreate
	rpCreate, rpCreateDiags := restartPolicyFromModel(ctx, plan.RestartPolicy)
	resp.Diagnostics.Append(rpCreateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	createInput.RestartPolicySwarm = rpCreate
	if err := r.client.UpdateApplication(ctx, plan.ID.ValueString(), createInput); err != nil {
		resp.Diagnostics.AddError("Error configuring application", err.Error())
		return
	}

	if err := r.configureAndDeploy(ctx, &plan, createTimeout); err != nil {
		// The application exists; persist its id so a later apply can retry.
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Application deploy failed", err.Error())
		return
	}

	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *applicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state applicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.GetApplication(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading application", err.Error())
		return
	}

	state.Name = types.StringValue(app.Name)
	state.EnvironmentID = types.StringValue(app.EnvironmentID)
	state.DockerImage = types.StringValue(app.DockerImage)
	state.AppName = types.StringValue(app.AppName)
	state.Status = types.StringValue(app.ApplicationStatus)
	if app.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(app.Description)
	}
	if app.RegistryURL != "" || !state.RegistryURL.IsNull() {
		state.RegistryURL = types.StringValue(app.RegistryURL)
	}
	if app.Username != "" || !state.RegistryUsername.IsNull() {
		state.RegistryUsername = types.StringValue(app.Username)
	}
	if app.ServerID != nil {
		state.ServerID = types.StringValue(*app.ServerID)
	} else {
		state.ServerID = types.StringNull()
	}
	// registry_password is intentionally NOT updated: the API does not return it.
	// Always populate env from the API so import works correctly.
	if app.Env != "" {
		envMap, diags := types.MapValueFrom(ctx, types.StringType, client.DotenvToMap(app.Env))
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			state.Env = envMap
		}
	} else if !state.Env.IsNull() {
		state.Env = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	// Replicas
	if app.Replicas != nil {
		state.Replicas = types.Int64Value(int64(*app.Replicas))
	} else {
		state.Replicas = types.Int64Null()
	}
	// HealthCheck
	hcObj, hcDiags := healthCheckToObject(ctx, app.HealthCheckSwarm)
	resp.Diagnostics.Append(hcDiags...)
	if !hcDiags.HasError() {
		// Only overwrite if API returned a value or state already had one
		if app.HealthCheckSwarm != nil || !state.HealthCheck.IsNull() {
			state.HealthCheck = hcObj
		}
	}
	// RestartPolicy
	rpObj, rpDiags := restartPolicyToObject(app.RestartPolicySwarm)
	resp.Diagnostics.Append(rpDiags...)
	if !rpDiags.HasError() {
		if app.RestartPolicySwarm != nil || !state.RestartPolicy.IsNull() {
			state.RestartPolicy = rpObj
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *applicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan applicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultDeployTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the update input including name/description + swarm advanced fields.
	updateInput := client.ApplicationInput{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}
	if !plan.Replicas.IsNull() && !plan.Replicas.IsUnknown() {
		v := int(plan.Replicas.ValueInt64())
		updateInput.Replicas = &v
	}
	hcUpdate, hcUpdateDiags := healthCheckFromModel(ctx, plan.HealthCheck)
	resp.Diagnostics.Append(hcUpdateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateInput.HealthCheckSwarm = hcUpdate
	rpUpdate, rpUpdateDiags := restartPolicyFromModel(ctx, plan.RestartPolicy)
	resp.Diagnostics.Append(rpUpdateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateInput.RestartPolicySwarm = rpUpdate

	if err := r.client.UpdateApplication(ctx, plan.ID.ValueString(), updateInput); err != nil {
		resp.Diagnostics.AddError("Error updating application", err.Error())
		return
	}

	if err := r.configureAndDeploy(ctx, &plan, updateTimeout); err != nil {
		plan.Status = types.StringValue("error")
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		resp.Diagnostics.AddError("Application deploy failed", err.Error())
		return
	}

	plan.Status = types.StringValue("done")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *applicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state applicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteApplication(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting application", err.Error())
	}
}

func (r *applicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
