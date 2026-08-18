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
	_ datasource.DataSource              = &NetworkDataSource{}
	_ datasource.DataSourceWithConfigure = &NetworkDataSource{}
)

func NewNetworkDataSource() datasource.DataSource {
	return &NetworkDataSource{}
}

// NetworkDataSource implements the netmaker_network data source.
type NetworkDataSource struct {
	client *nmclient.Client
}

func (d *NetworkDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (d *NetworkDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up an existing Netmaker network by name.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Computed: true},
			"name":                   schema.StringAttribute{Required: true, Description: "Network name to look up."},
			"address_range":          schema.StringAttribute{Computed: true},
			"address_range6":         schema.StringAttribute{Computed: true},
			"default_keepalive":      schema.Int64Attribute{Computed: true},
			"default_mtu":            schema.Int64Attribute{Computed: true},
			"auto_join":              schema.BoolAttribute{Computed: true},
			"auto_remove":            schema.BoolAttribute{Computed: true},
			"auto_remove_threshold":  schema.Int64Attribute{Computed: true},
			"jit_enabled":            schema.BoolAttribute{Computed: true},
			"default_value":          schema.StringAttribute{Computed: true},
			"default_token":          schema.StringAttribute{Computed: true, Sensitive: true},
			"default_enrollment_key": defaultEnrollmentKeyDataSourceSchema(),
		},
	}
}

// defaultEnrollmentKeyDataSourceSchema is the read-only counterpart of the
// nested default_enrollment_key attribute on the netmaker_network resource
// (network_resource.go) — same editable fields, all Computed since data
// sources can't accept configuration for them. Its value/token are
// separate top-level attributes for the same reason as on the resource
// (see NetworkResourceModel's doc comment): nesting a Sensitive leaf in a
// partially-known object trips a terraform-plugin-framework consistency
// check.
func defaultEnrollmentKeyDataSourceSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: "The network's auto-created default enrollment key's editable settings.",
		Computed:    true,
		Attributes: map[string]schema.Attribute{
			"gateway_id": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the relay/gateway node devices enrolled with this key are auto-relayed through, if any.",
			},
			"auto_assign_gateway": schema.BoolAttribute{Computed: true},
			"tags": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *NetworkDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NetworkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model NetworkResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	n, err := d.client.GetNetwork(ctx, model.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading network", err.Error())
		return
	}

	defaultKey, err := d.client.GetDefaultEnrollmentKeyForNetwork(ctx, n.NetID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading network's default enrollment key", err.Error())
		return
	}

	out := networkToModel(n)
	resp.Diagnostics.Append(applyDefaultEnrollmentKey(ctx, &out, defaultKey)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, out)...)
}
