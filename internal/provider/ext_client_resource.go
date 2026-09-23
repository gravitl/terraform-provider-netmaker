package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
	"github.com/gravitl/terraform-provider-netmaker/internal/provisioner"
)

var (
	_ resource.Resource               = &ExtClientResource{}
	_ resource.ResourceWithConfigure  = &ExtClientResource{}
	_ resource.ResourceWithModifyPlan = &ExtClientResource{}
)

func NewExtClientResource() resource.Resource {
	return &ExtClientResource{}
}

// ExtClientResource implements the netmaker_ext_client resource: creates a
// remote-access WireGuard client attached to an existing ingress gateway
// node, and deploys it via `mode` (SSH today — see ModeModel) —
// installing wireguard-tools if needed and bringing up the
// interface/tunnel from the server-rendered config.
// Creating/managing the gateway node itself is out of scope; gateway_node_id
// must reference an existing ingress gateway (see the netmaker_node data
// source's is_ingress_gateway attribute).
type ExtClientResource struct {
	client *nmclient.Client
}

// ExtClientResourceModel maps the netmaker_ext_client resource schema.
type ExtClientResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Network         types.String `tfsdk:"network"`
	GatewayNodeID   types.String `tfsdk:"gateway_node_id"`
	DNS             types.String `tfsdk:"dns"`
	ExtraAllowedIPs types.List   `tfsdk:"extra_allowed_ips"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	Tags            types.List   `tfsdk:"tags"`
	PostUp          types.String `tfsdk:"post_up"`
	PostDown        types.String `tfsdk:"post_down"`
	DeviceName      types.String `tfsdk:"device_name"`
	Address         types.String `tfsdk:"address"`
	Address6        types.String `tfsdk:"address6"`
	PublicKey       types.String `tfsdk:"public_key"`
	PrivateKey      types.String `tfsdk:"private_key"`
	Mode            *ModeModel   `tfsdk:"mode"`
}

func (r *ExtClientResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ext_client"
}

func (r *ExtClientResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates a Netmaker ext client (remote-access WireGuard config) attached to an existing ingress gateway node, and deploys it onto a target machine over SSH (installs wireguard-tools if needed, brings up the interface/tunnel).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Server-assigned ext client ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"network": schema.StringAttribute{
				Description: "Network the gateway node belongs to. Changing this forces recreation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"gateway_node_id": schema.StringAttribute{
				Description: "ID of an existing ingress gateway node in the given network (see netmaker_node's is_ingress_gateway). Changing this forces recreation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dns":         schema.StringAttribute{Optional: true, Computed: true},
			"enabled":     schema.BoolAttribute{Optional: true, Computed: true},
			"post_up":     schema.StringAttribute{Optional: true, Computed: true},
			"post_down":   schema.StringAttribute{Optional: true, Computed: true},
			"device_name": schema.StringAttribute{Optional: true, Computed: true},
			"extra_allowed_ips": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"tags": schema.ListAttribute{
				Description: "Tags to apply to this ext client.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"address":  schema.StringAttribute{Computed: true},
			"address6": schema.StringAttribute{Computed: true},
			"public_key": schema.StringAttribute{
				Computed: true,
			},
			"private_key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"mode": modeSchema(),
		},
	}
}

func (r *ExtClientResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ExtClientResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return // destroy
	}
	var plan ExtClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || plan.Mode == nil {
		return
	}
	if err := plan.Mode.validate(); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("mode"), "Invalid deploy configuration", err.Error())
	}
}

func (r *ExtClientResource) buildRequest(ctx context.Context, plan ExtClientResourceModel) (*nmclient.ExtClientRequest, error) {
	var extraAllowedIPs []string
	if !plan.ExtraAllowedIPs.IsNull() && !plan.ExtraAllowedIPs.IsUnknown() {
		if err := plan.ExtraAllowedIPs.ElementsAs(ctx, &extraAllowedIPs, false); err != nil {
			return nil, fmt.Errorf("invalid extra_allowed_ips: %v", err)
		}
	}
	tags, err := r.resolveTagSet(ctx, plan.Network.ValueString(), plan.Tags)
	if err != nil {
		return nil, err
	}

	return &nmclient.ExtClientRequest{
		DNS:             plan.DNS.ValueString(),
		ExtraAllowedIPs: extraAllowedIPs,
		Enabled:         plan.Enabled.ValueBool(),
		Tags:            tags,
		PostUp:          plan.PostUp.ValueString(),
		PostDown:        plan.PostDown.ValueString(),
		DeviceName:      plan.DeviceName.ValueString(),
	}, nil
}

// resolveTagSet validates that each tag name in l already exists as a
// netmaker_tag in network, and returns the fully-qualified tag IDs
// ("<network>.<name>") as a set, matching ExtClient.Tags' map[TagID]struct{}
// shape server-side. Like netmaker_enrollment_key.tags (see its doc
// comment), Netmaker's own API doesn't validate this — a tag referenced
// here that was never created would otherwise silently persist as a
// broken reference — so this resource fails instead.
func (r *ExtClientResource) resolveTagSet(ctx context.Context, network string, l types.List) (map[string]struct{}, error) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var names []string
	if err := l.ElementsAs(ctx, &names, false); err != nil {
		return nil, fmt.Errorf("invalid tags: %v", err)
	}
	if len(names) == 0 {
		return nil, nil
	}

	existing, err := r.client.ListTags(ctx, network)
	if err != nil {
		return nil, fmt.Errorf("listing tags for network %q: %w", network, err)
	}
	existingNames := make(map[string]struct{}, len(existing))
	for _, t := range existing {
		existingNames[t.TagName] = struct{}{}
	}

	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, ok := existingNames[name]; !ok {
			return nil, fmt.Errorf("tag %q does not exist in network %q — create it with a netmaker_tag resource first (Netmaker does not auto-create tags)", name, network)
		}
		set[nmclient.TagID(network, name)] = struct{}{}
	}
	return set, nil
}

// tagSetToStringList converts a server-returned ExtClient.Tags set (keyed
// by fully-qualified tag ID, "<network>.<name>") back to the plain tag
// names this resource's "tags" attribute is configured with, stripping
// the "<network>." prefix. Network and tag names can't contain literal
// dots (proLogic.CheckIDSyntax), so the prefix is unambiguous to strip.
func tagSetToStringList(ctx context.Context, network string, set map[string]struct{}) (types.List, diag.Diagnostics) {
	prefix := network + "."
	tags := make([]string, 0, len(set))
	for id := range set {
		tags = append(tags, strings.TrimPrefix(id, prefix))
	}
	return types.ListValueFrom(ctx, types.StringType, tags)
}

func (r *ExtClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ExtClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqBody, err := r.buildRequest(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ext client configuration", err.Error())
		return
	}

	created, err := r.client.CreateExtClient(ctx, plan.Network.ValueString(), plan.GatewayNodeID.ValueString(), reqBody)
	if err != nil {
		resp.Diagnostics.AddError("Error creating ext client", err.Error())
		return
	}

	conf, err := r.client.GetExtClientConfigFile(ctx, plan.Network.ValueString(), created.ClientID)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching ext client config", err.Error())
		return
	}

	connCfg, err := plan.Mode.toProvisionerConfig()
	if err != nil {
		resp.Diagnostics.AddError("Invalid deploy configuration", err.Error())
		return
	}
	conn, err := provisioner.Connect(connCfg)
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to target machine", err.Error())
		return
	}
	defer conn.Close()

	info, err := provisioner.Detect(conn)
	if err != nil {
		resp.Diagnostics.AddError("Error detecting target OS", err.Error())
		return
	}

	if err := provisioner.ApplyExtClientConfig(conn, info, provisioner.InterfaceName(created.ClientID), conf); err != nil {
		resp.Diagnostics.AddError("Error applying WireGuard config", err.Error())
		return
	}

	model, diags := extClientResourceToModel(ctx, created, plan.Mode)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *ExtClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ExtClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.client.GetExtClient(ctx, state.Network.ValueString(), state.ID.ValueString())
	if err != nil {
		var apiErr *nmclient.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading ext client", err.Error())
		return
	}

	model, diags := extClientResourceToModel(ctx, client, state.Mode)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *ExtClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ExtClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqBody, err := r.buildRequest(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ext client configuration", err.Error())
		return
	}

	updated, err := r.client.UpdateExtClient(ctx, plan.Network.ValueString(), plan.ID.ValueString(), reqBody)
	if err != nil {
		resp.Diagnostics.AddError("Error updating ext client", err.Error())
		return
	}

	model, diags := extClientResourceToModel(ctx, updated, plan.Mode)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *ExtClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ExtClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Mode != nil {
		connCfg, cfgErr := state.Mode.toProvisionerConfig()
		if cfgErr != nil {
			resp.Diagnostics.AddWarning("Invalid deploy configuration", "Proceeding to delete the ext client server-side without tearing down WireGuard locally: "+cfgErr.Error())
		} else if conn, err := provisioner.Connect(connCfg); err != nil {
			resp.Diagnostics.AddWarning(
				"Could not reach target machine to tear down WireGuard",
				"The machine may already be terminated. Proceeding to delete the ext client server-side: "+err.Error(),
			)
		} else {
			defer conn.Close()
			if info, err := provisioner.Detect(conn); err != nil {
				resp.Diagnostics.AddWarning("Error detecting target OS", err.Error())
			} else if err := provisioner.TeardownExtClientConfig(conn, info, provisioner.InterfaceName(state.ID.ValueString())); err != nil {
				resp.Diagnostics.AddWarning("Error tearing down WireGuard config", err.Error())
			}
		}
	}

	if err := r.client.DeleteExtClient(ctx, state.Network.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting ext client", err.Error())
	}
}

func extClientResourceToModel(ctx context.Context, c *nmclient.ExtClient, mode *ModeModel) (ExtClientResourceModel, diag.Diagnostics) {
	extraAllowedIPs, diags := types.ListValueFrom(ctx, types.StringType, c.ExtraAllowedIPs)
	tags, tagDiags := tagSetToStringList(ctx, c.Network, c.Tags)
	diags.Append(tagDiags...)

	return ExtClientResourceModel{
		ID:              types.StringValue(c.ClientID),
		Network:         types.StringValue(c.Network),
		GatewayNodeID:   types.StringValue(c.IngressGatewayID),
		DNS:             types.StringValue(c.DNS),
		ExtraAllowedIPs: extraAllowedIPs,
		Enabled:         types.BoolValue(c.Enabled),
		Tags:            tags,
		PostUp:          types.StringValue(c.PostUp),
		PostDown:        types.StringValue(c.PostDown),
		DeviceName:      types.StringValue(c.DeviceName),
		Address:         types.StringValue(c.Address),
		Address6:        types.StringValue(c.Address6),
		PublicKey:       types.StringValue(c.PublicKey),
		PrivateKey:      types.StringValue(c.PrivateKey),
		Mode:            mode,
	}, diags
}
