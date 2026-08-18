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
	_ datasource.DataSource              = &NodeDataSource{}
	_ datasource.DataSourceWithConfigure = &NodeDataSource{}
)

func NewNodeDataSource() datasource.DataSource {
	return &NodeDataSource{}
}

// NodeDataSource implements the netmaker_node data source. A Node is a
// device's per-network membership record — see nm-api-client-go's Node
// doc comment: there is no direct create/get-by-id endpoint, so this looks
// up a node from ListNodesByNetwork by ID.
type NodeDataSource struct {
	client *nmclient.Client
}

// NodeModel maps a Node to Terraform-facing attributes, shared between the
// netmaker_node and netmaker_nodes data sources.
type NodeModel struct {
	ID                types.String `tfsdk:"id"`
	DeviceID          types.String `tfsdk:"device_id"`
	Network           types.String `tfsdk:"network"`
	Address           types.String `tfsdk:"address"`
	Address6          types.String `tfsdk:"address6"`
	Connected         types.Bool   `tfsdk:"connected"`
	IsEgressGateway   types.Bool   `tfsdk:"is_egress_gateway"`
	IsIngressGateway  types.Bool   `tfsdk:"is_ingress_gateway"`
	IsRelay           types.Bool   `tfsdk:"is_relay"`
	IsInternetGateway types.Bool   `tfsdk:"is_internet_gateway"`
	Status            types.String `tfsdk:"status"`
}

func nodeModelSchema(idDescription string) map[string]schema.Attribute {
	idAttr := schema.StringAttribute{Computed: true}
	if idDescription != "" {
		idAttr = schema.StringAttribute{Required: true, Description: idDescription}
	}
	return map[string]schema.Attribute{
		"id":                  idAttr,
		"device_id":           schema.StringAttribute{Computed: true, Description: "ID of the Device this node belongs to."},
		"network":             schema.StringAttribute{Computed: true},
		"address":             schema.StringAttribute{Computed: true},
		"address6":            schema.StringAttribute{Computed: true},
		"connected":           schema.BoolAttribute{Computed: true},
		"is_egress_gateway":   schema.BoolAttribute{Computed: true},
		"is_ingress_gateway":  schema.BoolAttribute{Computed: true},
		"is_relay":            schema.BoolAttribute{Computed: true},
		"is_internet_gateway": schema.BoolAttribute{Computed: true},
		"status":              schema.StringAttribute{Computed: true},
	}
}

func nodeToModel(n *nmclient.Node) NodeModel {
	return NodeModel{
		ID:                types.StringValue(n.ID),
		DeviceID:          types.StringValue(n.HostID),
		Network:           types.StringValue(n.Network),
		Address:           types.StringValue(n.Address),
		Address6:          types.StringValue(n.Address6),
		Connected:         types.BoolValue(n.Connected),
		IsEgressGateway:   types.BoolValue(n.IsEgressGateway),
		IsIngressGateway:  types.BoolValue(n.IsIngressGateway),
		IsRelay:           types.BoolValue(n.IsRelay),
		IsInternetGateway: types.BoolValue(n.IsInternetGateway),
		Status:            types.StringValue(n.Status),
	}
}

func (d *NodeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node"
}

func (d *NodeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := nodeModelSchema("Node ID to look up.")
	attrs["network"] = schema.StringAttribute{Required: true, Description: "Network the node belongs to."}
	resp.Schema = schema.Schema{
		Description: "Looks up an existing Netmaker node by network and ID.",
		Attributes:  attrs,
	}
}

func (d *NodeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NodeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model NodeModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	node, err := d.client.GetNode(ctx, model.Network.ValueString(), model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading node", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, nodeToModel(node))...)
}
