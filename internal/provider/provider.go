package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func (p *dokployProvider) Configure(_ context.Context, _ provider.ConfigureRequest, _ *provider.ConfigureResponse) {
	// Implemented in a later task.
}

func (p *dokployProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil // Populated in a later task.
}

func (p *dokployProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
