package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var (
	_ datasource.DataSource              = &organizationDataSource{}
	_ datasource.DataSourceWithConfigure = &organizationDataSource{}
)

type organizationDataSource struct {
	client *client.Client
}

// NewOrganizationDataSource is the data source constructor registered by the provider.
func NewOrganizationDataSource() datasource.DataSource {
	return &organizationDataSource{}
}

type organizationDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Slug types.String `tfsdk:"slug"`
}

func (d *organizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *organizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Dokploy organization. If the API key can see more than one organization, set `name` to select one.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization identifier.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization name. Optional when the API key sees exactly one organization; required to disambiguate when it sees several.",
			},
			"slug": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization slug (may be empty).",
			},
		},
	}
}

func (d *organizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	d.client = c
}

func (d *organizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config organizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgs, err := d.client.ListOrganizations(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}
	if len(orgs) == 0 {
		resp.Diagnostics.AddError("No organization found",
			"The configured API key is not associated with any organization.")
		return
	}

	names := make([]string, len(orgs))
	for i, o := range orgs {
		names[i] = o.Name
	}

	var chosen *client.Organization
	if !config.Name.IsNull() && config.Name.ValueString() != "" {
		want := config.Name.ValueString()
		for i := range orgs {
			if orgs[i].Name == want {
				chosen = &orgs[i]
				break
			}
		}
		if chosen == nil {
			resp.Diagnostics.AddError("Organization not found",
				fmt.Sprintf("No organization named %q. Available: %s.", want, strings.Join(names, ", ")))
			return
		}
	} else {
		if len(orgs) > 1 {
			resp.Diagnostics.AddError("Multiple organizations found",
				fmt.Sprintf("The API key can see %d organizations (%s). Set the `name` argument to select one.",
					len(orgs), strings.Join(names, ", ")))
			return
		}
		chosen = &orgs[0]
	}

	state := organizationDataSourceModel{
		ID:   types.StringValue(chosen.ID),
		Name: types.StringValue(chosen.Name),
		Slug: types.StringValue(chosen.Slug),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
