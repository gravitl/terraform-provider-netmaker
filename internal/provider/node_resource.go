package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
)

var (
	_ resource.Resource              = &NodeResource{}
	_ resource.ResourceWithConfigure = &NodeResource{}
)

func NewNodeResource() resource.Resource {
	return &NodeResource{}
}

// NodeResource implements the netmaker_node resource: joining an existing
// device to an additional network. This is pure control-plane (no SSH) —
// the device is already running netclient and picks up the new network via
// its existing MQTT/pull connection once the Node exists server-side.
type NodeResource struct {
	client *nmclient.Client
}

// NodeResourceModel maps the netmaker_node resource schema.
type NodeResourceModel struct {
	ID                types.String `tfsdk:"id"`
	DeviceID          types.String `tfsdk:"device_id"`
	Network           types.String `tfsdk:"network"`
	ForceDelete       types.Bool   `tfsdk:"force_delete"`
	Address           types.String `tfsdk:"address"`
	Address6          types.String `tfsdk:"address6"`
	Connected         types.Bool   `tfsdk:"connected"`
	IsEgressGateway   types.Bool   `tfsdk:"is_egress_gateway"`
	IsIngressGateway  types.Bool   `tfsdk:"is_ingress_gateway"`
	IsRelay           types.Bool   `tfsdk:"is_relay"`
	IsInternetGateway types.Bool   `tfsdk:"is_internet_gateway"`
	Status            types.String `tfsdk:"status"`
}

func (r *NodeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node"
}

func (r *NodeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Joins an existing netmaker_device to a network, creating a Node. Does not install or configure anything on the device — it must already be running netclient (e.g. via netmaker_device); the running netclient picks up the new network automatically. Optionally turns the node into an ingress gateway (is_ingress_gateway) — the attach point for netmaker_ext_client's gateway_node_id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Server-assigned node ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"device_id": schema.StringAttribute{
				Description: "ID of the device (netmaker_device) to join to the network.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"network": schema.StringAttribute{
				Description: "Network to join.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"force_delete": schema.BoolAttribute{
				Description: "Remove the node even if it has unresolved dependents (e.g. it's a relay/gateway other nodes depend on).",
				Optional:    true,
			},
			"address":           schema.StringAttribute{Computed: true},
			"address6":          schema.StringAttribute{Computed: true},
			"connected":         schema.BoolAttribute{Computed: true},
			"is_egress_gateway": schema.BoolAttribute{Computed: true},
			"is_ingress_gateway": schema.BoolAttribute{
				Description: "Whether this node is an ingress (remote-access) gateway — the attach point for netmaker_ext_client's gateway_node_id. Set to true to turn this node into a gateway on create/update; set to false to remove the gateway role.",
				Optional:    true,
				Computed:    true,
			},
			"is_relay":            schema.BoolAttribute{Computed: true},
			"is_internet_gateway": schema.BoolAttribute{Computed: true},
			"status":              schema.StringAttribute{Computed: true},
		},
	}
}

func (r *NodeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected resource configure type", fmt.Sprintf("expected *providerData, got %T", req.ProviderData))
		return
	}
	r.client = data.client
}

func (r *NodeResource) findNode(ctx context.Context, network, deviceID string) (*nmclient.Node, error) {
	nodes, err := r.client.ListNodesByNetwork(ctx, network)
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		if nodes[i].HostID == deviceID {
			return &nodes[i], nil
		}
	}
	return nil, nil
}

func (r *NodeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NodeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deviceID := plan.DeviceID.ValueString()
	network := plan.Network.ValueString()

	if err := r.client.AddDeviceToNetwork(ctx, deviceID, network); err != nil {
		resp.Diagnostics.AddError("Error joining device to network", err.Error())
		return
	}

	node, err := r.findNode(ctx, network, deviceID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading created node", err.Error())
		return
	}
	if node == nil {
		resp.Diagnostics.AddError("Node not found after join", fmt.Sprintf("device %q did not appear to join network %q", deviceID, network))
		return
	}

	if !plan.IsIngressGateway.IsNull() && !plan.IsIngressGateway.IsUnknown() && plan.IsIngressGateway.ValueBool() {
		node, err = r.client.CreateGateway(ctx, network, node.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error creating gateway", err.Error())
			return
		}
	}

	model := nodeResourceToModel(node)
	model.ForceDelete = plan.ForceDelete
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *NodeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NodeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	node, err := r.findNode(ctx, state.Network.ValueString(), state.DeviceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading node", err.Error())
		return
	}
	if node == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	model := nodeResourceToModel(node)
	model.ForceDelete = state.ForceDelete
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *NodeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// device_id/network are RequiresReplace; force_delete has no
	// server-side effect until Delete. is_ingress_gateway is the only
	// attribute that needs reconciling here.
	var plan NodeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	network := plan.Network.ValueString()
	node, err := r.findNode(ctx, network, plan.DeviceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading node", err.Error())
		return
	}
	if node == nil {
		resp.Diagnostics.AddError("Node not found", fmt.Sprintf("device %q is no longer joined to network %q", plan.DeviceID.ValueString(), network))
		return
	}

	wantGateway := plan.IsIngressGateway.ValueBool()
	if wantGateway != node.IsIngressGateway {
		if wantGateway {
			node, err = r.client.CreateGateway(ctx, network, node.ID)
		} else {
			node, err = r.client.DeleteGateway(ctx, network, node.ID)
		}
		if err != nil {
			resp.Diagnostics.AddError("Error updating gateway status", err.Error())
			return
		}
	}

	model := nodeResourceToModel(node)
	model.ForceDelete = plan.ForceDelete
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *NodeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NodeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.RemoveDeviceFromNetwork(ctx, state.DeviceID.ValueString(), state.Network.ValueString(), state.ForceDelete.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("Error removing device from network", err.Error())
	}
}

func nodeResourceToModel(n *nmclient.Node) NodeResourceModel {
	return NodeResourceModel{
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
