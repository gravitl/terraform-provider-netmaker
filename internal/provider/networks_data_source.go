package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
)

var (
	_ datasource.DataSource              = &NetworksDataSource{}
	_ datasource.DataSourceWithConfigure = &NetworksDataSource{}
)

func NewNetworksDataSource() datasource.DataSource {
	return &NetworksDataSource{}
}

// NetworksDataSource implements the netmaker_networks data source, listing
// every network visible to the tenant.
type NetworksDataSource struct {
	client *nmclient.Client
}

type NetworksDataSourceModel struct {
	Networks []NetworkResourceModel `tfsdk:"networks"`
}

func (d *NetworksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_networks"
}

func (d *NetworksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all Netmaker networks visible to the tenant.",
		Attributes: map[string]schema.Attribute{
			"networks": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                     schema.StringAttribute{Computed: true},
						"name":                   schema.StringAttribute{Computed: true},
						"address_range":          schema.StringAttribute{Computed: true},
						"address_range6":         schema.StringAttribute{Computed: true},
						"auto_join":              schema.BoolAttribute{Computed: true},
						"auto_remove":            schema.BoolAttribute{Computed: true},
						"auto_remove_threshold":  schema.Int64Attribute{Computed: true},
						"auto_remove_tags":       schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"jit_enabled":            schema.BoolAttribute{Computed: true},
						"default_value":          schema.StringAttribute{Computed: true},
						"default_token":          schema.StringAttribute{Computed: true, Sensitive: true},
						"default_enrollment_key": defaultEnrollmentKeyDataSourceSchema(),
					},
				},
			},
		},
	}
}

func (d *NetworksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected data source configure type", fmt.Sprintf("expected *providerData, got %T", req.ProviderData))
		return
	}
	d.client = data.client
}

func (d *NetworksDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	networks, err := d.client.ListNetworks(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing networks", err.Error())
		return
	}

	model := NetworksDataSourceModel{Networks: make([]NetworkResourceModel, 0, len(networks))}
	for i := range networks {
		netModel, diags := networkToModel(ctx, &networks[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// One extra call per network to fetch its default enrollment key —
		// acceptable at this scale; revisit if this list ever needs to
		// handle a large number of networks efficiently.
		defaultKey, err := d.client.GetDefaultEnrollmentKeyForNetwork(ctx, networks[i].NetID)
		if err != nil {
			resp.Diagnostics.AddError("Error reading default enrollment key for network "+networks[i].NetID, err.Error())
			return
		}
		resp.Diagnostics.Append(applyDefaultEnrollmentKey(ctx, &netModel, defaultKey)...)
		if resp.Diagnostics.HasError() {
			return
		}

		model.Networks = append(model.Networks, netModel)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
