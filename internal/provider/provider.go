//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name dokploy --provider-dir ../../

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

// Ensure dokployProvider satisfies the provider.Provider interface.
var _ provider.Provider = &dokployProvider{}

type dokployProvider struct {
	version string
}

// New returns a provider factory used by main.go and acceptance tests.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &dokployProvider{version: version}
	}
}

// dokployProviderModel maps the provider configuration block.
type dokployProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func (p *dokployProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "dokploy"
	resp.Version = p.version
}

func (p *dokployProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Dokploy provider manages resources on a Dokploy instance.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Base URL of the Dokploy instance, e.g. `https://dokploy.example.com`. May also be set via the `DOKPLOY_ENDPOINT` environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Dokploy API key sent as the `x-api-key` header. May also be set via the `DOKPLOY_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *dokployProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config dokployProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := config.Endpoint.ValueString()
	if endpoint == "" {
		endpoint = getEnv("DOKPLOY_ENDPOINT")
	}
	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = getEnv("DOKPLOY_API_KEY")
	}

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"),
			"Missing Dokploy endpoint",
			"Set the `endpoint` attribute or the DOKPLOY_ENDPOINT environment variable.")
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"),
			"Missing Dokploy API key",
			"Set the `api_key` attribute or the DOKPLOY_API_KEY environment variable.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c := client.New(endpoint, apiKey)
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *dokployProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewEnvironmentResource,
		NewApplicationResource,
		NewDomainResource,
		NewPostgresResource,
		NewMysqlResource,
		NewMariadbResource,
		NewMongoResource,
		NewRedisResource,
		NewDestinationResource,
	}
}

func (p *dokployProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrganizationDataSource,
	}
}

// getEnv wraps os.Getenv so tests can reference it through one symbol.
func getEnv(key string) string {
	return os.Getenv(key)
}
