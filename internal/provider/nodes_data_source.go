package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
)

var (
	_ datasource.DataSource              = &NodesDataSource{}
	_ datasource.DataSourceWithConfigure = &NodesDataSource{}
)

func NewNodesDataSource() datasource.DataSource {
	return &NodesDataSource{}
}

// NodesDataSource implements the netmaker_nodes data source. With no
// network filter set, it lists nodes across all networks; with one set, it
// lists only that network's nodes.
type NodesDataSource struct {
	client *nmclient.Client
}

type NodesDataSourceModel struct {
	Network *string     `tfsdk:"network"`
	Nodes   []NodeModel `tfsdk:"nodes"`
}

func (d *NodesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nodes"
}

func (d *NodesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Netmaker nodes, optionally filtered to a single network.",
		Attributes: map[string]schema.Attribute{
			"network": schema.StringAttribute{
				Optional:    true,
				Description: "If set, only list nodes in this network; otherwise list nodes across all networks.",
			},
			"nodes": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: nodeModelSchema(""),
				},
			},
		},
	}
}

func (d *NodesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NodesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model NodesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var nodes []nmclient.Node
	var err error
	if model.Network != nil && *model.Network != "" {
		nodes, err = d.client.ListNodesByNetwork(ctx, *model.Network)
	} else {
		nodes, err = d.client.ListNodes(ctx)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error listing nodes", err.Error())
		return
	}

	model.Nodes = make([]NodeModel, 0, len(nodes))
	for i := range nodes {
		model.Nodes = append(model.Nodes, nodeToModel(&nodes[i]))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
