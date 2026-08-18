package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
)

var (
	_ resource.Resource                = &NetworkResource{}
	_ resource.ResourceWithConfigure   = &NetworkResource{}
	_ resource.ResourceWithImportState = &NetworkResource{}
)

func NewNetworkResource() resource.Resource {
	return &NetworkResource{}
}

// NetworkResource implements the netmaker_network resource.
type NetworkResource struct {
	client *nmclient.Client
}

// NetworkResourceModel maps the netmaker_network resource schema.
//
// DefaultValue/Token are kept as top-level attributes rather
// than nested inside DefaultEnrollmentKey, even though they're part of the
// same underlying object: nesting a Sensitive leaf (token) inside an
// object that's only partially known at plan time (editable fields like
// tags are known from config, but value/token are Computed) trips a
// terraform-plugin-framework/Terraform-core consistency-check bug —
// "Provider produced inconsistent result after apply ... inconsistent
// values for sensitive attribute" — that doesn't affect top-level
// Sensitive attributes.
type NetworkResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	AddressRange        types.String `tfsdk:"address_range"`
	AddressRange6       types.String `tfsdk:"address_range6"`
	DefaultKeepAlive    types.Int64  `tfsdk:"default_keepalive"`
	DefaultMTU          types.Int64  `tfsdk:"default_mtu"`
	AutoJoin            types.Bool   `tfsdk:"auto_join"`
	AutoRemove          types.Bool   `tfsdk:"auto_remove"`
	AutoRemoveThreshold types.Int64  `tfsdk:"auto_remove_threshold"`
	JITEnabled          types.Bool   `tfsdk:"jit_enabled"`
	DefaultValue        types.String `tfsdk:"default_value"`
	DefaultToken        types.String `tfsdk:"default_token"`
	// DefaultEnrollmentKey is types.Object, not a plain Go struct pointer,
	// because it must be able to represent "unknown" (e.g. when a
	// netmaker_network is created/updated without setting
	// default_enrollment_key at all — Optional+Computed, so its plan value
	// is genuinely unknown, not null) — a raw struct pointer can only
	// represent null or a fully-populated value, and decoding an unknown
	// plan value into one panics with "Received unknown value, however the
	// target type cannot handle unknown values."
	DefaultEnrollmentKey types.Object `tfsdk:"default_enrollment_key"`
}

// DefaultEnrollmentKeyModel maps the netmaker_network resource's nested
// default_enrollment_key attribute — Netmaker auto-creates an unlimited
// enrollment key scoped to the network whenever the network is created
// (logic.CreateDefaultNetworkEnrollmentKey, controllers/network.go), and
// deletes it automatically when the network is deleted. Rather than
// requiring a separate netmaker_enrollment_key resource plus a manual
// `terraform import` for that key, this manages it in place as part of the
// network resource. Its value/token live at the top level of
// NetworkResourceModel instead (see that type's doc comment).
type DefaultEnrollmentKeyModel struct {
	GatewayID         types.String `tfsdk:"gateway_id"`
	AutoAssignGateway types.Bool   `tfsdk:"auto_assign_gateway"`
	Tags              types.List   `tfsdk:"tags"`
}

var defaultEnrollmentKeyAttrTypes = map[string]attr.Type{
	"gateway_id":          types.StringType,
	"auto_assign_gateway": types.BoolType,
	"tags":                types.ListType{ElemType: types.StringType},
}

func (r *NetworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *NetworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Netmaker network.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Server-assigned network ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Network name (Netmaker's netid). 1-32 characters. Changing this forces recreation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"address_range": schema.StringAttribute{
				Description: "IPv4 CIDR pool for this network, e.g. 10.10.0.0/16. At least one of address_range/address_range6 is required.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"address_range6": schema.StringAttribute{
				Description: "IPv6 CIDR pool for this network. At least one of address_range/address_range6 is required.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"default_keepalive": schema.Int64Attribute{
				Description: "Default WireGuard persistent keepalive, in seconds.",
				Optional:    true,
				Computed:    true,
			},
			"default_mtu": schema.Int64Attribute{
				Description: "Default WireGuard interface MTU.",
				Optional:    true,
				Computed:    true,
			},
			"auto_join": schema.BoolAttribute{
				Description: "Whether new enrollment keys default to auto-joining this network.",
				Optional:    true,
				Computed:    true,
			},
			"auto_remove": schema.BoolAttribute{
				Description: "Whether inactive nodes are automatically removed from this network.",
				Optional:    true,
				Computed:    true,
			},
			"auto_remove_threshold": schema.Int64Attribute{
				Description: "Minutes of inactivity before auto-remove applies.",
				Optional:    true,
				Computed:    true,
			},
			"jit_enabled": schema.BoolAttribute{
				Description: "Whether just-in-time access is enabled for this network.",
				Optional:    true,
				Computed:    true,
			},
			"default_value": schema.StringAttribute{
				Description: "Server-assigned value of the network's default enrollment key.",
				Computed:    true,
			},
			"default_token": schema.StringAttribute{
				Description: "Base64-encoded join token for the network's default enrollment key, to pass to `netclient join -t`.",
				Computed:    true,
				Sensitive:   true,
			},
			"default_enrollment_key": schema.SingleNestedAttribute{
				Description: "Netmaker auto-creates an unlimited enrollment key scoped to this network whenever the network is created (and deletes it when the network is deleted) — this manages that key's editable settings in place rather than requiring a separate netmaker_enrollment_key resource plus a manual import; see default_value/default_token for its value/token. Omit to leave its settings as Netmaker's defaults (no gateway, no tags). Additional independent keys for this network can still be declared as separate netmaker_enrollment_key resources.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"gateway_id": schema.StringAttribute{
						Description: "ID of the relay/gateway node devices enrolled with this key are auto-relayed through, if any.",
						Optional:    true,
						Computed:    true,
					},
					"auto_assign_gateway": schema.BoolAttribute{
						Description: "Whether devices enrolled with this key auto-select a gateway.",
						Optional:    true,
						Computed:    true,
					},
					"tags": schema.ListAttribute{
						Description: "Tags to apply to devices enrolled with this key.",
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
					},
				},
			},
		},
	}
}

func (r *NetworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	n := &nmclient.Network{
		NetID:               plan.Name.ValueString(),
		AddressRange:        plan.AddressRange.ValueString(),
		AddressRange6:       plan.AddressRange6.ValueString(),
		DefaultKeepAlive:    int(plan.DefaultKeepAlive.ValueInt64()),
		DefaultMTU:          int32(plan.DefaultMTU.ValueInt64()),
		AutoJoin:            plan.AutoJoin.ValueBool(),
		AutoRemove:          plan.AutoRemove.ValueBool(),
		AutoRemoveThreshold: int(plan.AutoRemoveThreshold.ValueInt64()),
		JITEnabled:          plan.JITEnabled.ValueBool(),
	}

	created, err := r.client.CreateNetwork(ctx, n)
	if err != nil {
		resp.Diagnostics.AddError("Error creating network", err.Error())
		return
	}

	defaultKey, err := r.client.GetDefaultEnrollmentKeyForNetwork(ctx, created.NetID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading network's auto-created default enrollment key", err.Error())
		return
	}

	cfg, diags := decodeDefaultEnrollmentKeyConfig(ctx, plan.DefaultEnrollmentKey)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg != nil {
		defaultKey, err = r.applyDefaultEnrollmentKeyConfig(ctx, created.NetID, defaultKey.Value, cfg)
		if err != nil {
			resp.Diagnostics.AddError("Error configuring network's default enrollment key", err.Error())
			return
		}
	}

	model := networkToModel(created)
	resp.Diagnostics.Append(applyDefaultEnrollmentKey(ctx, &model, defaultKey)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

// applyDefaultEnrollmentKeyConfig updates the network's auto-created
// default enrollment key's mutable fields (gateway/auto-assign-gateway/
// tags) to match cfg, preserving the key's identity as an unlimited,
// default key scoped only to this network.
//
// Two server-side quirks (logic.UpdateEnrollmentKey) matter here:
//   - the request's Tags field is only used at key *creation* as the
//     display name (tags[0]); the actually-persisted, readable Tags value
//     is driven by the request's Groups field instead, both at creation
//     and on update — so tags must be sent as Groups, not Tags.
//   - Default is not preserved implicitly: if the request omits it (or
//     sends false) while the key is currently the default, the server
//     unsets Default. Since this is specifically the network's default
//     key, Default: true must always be sent explicitly, or the key
//     silently stops being discoverable via
//     GetDefaultEnrollmentKeyForNetwork on the next read.
func (r *NetworkResource) applyDefaultEnrollmentKeyConfig(ctx context.Context, netID, keyValue string, cfg *DefaultEnrollmentKeyModel) (*nmclient.EnrollmentKeyDetail, error) {
	var tags []string
	if !cfg.Tags.IsNull() && !cfg.Tags.IsUnknown() {
		if err := cfg.Tags.ElementsAs(ctx, &tags, false); err != nil {
			return nil, fmt.Errorf("invalid default_enrollment_key.tags: %v", err)
		}
	}
	req := &nmclient.EnrollmentKeyRequest{
		Networks:          []string{netID},
		Unlimited:         true,
		Type:              nmclient.KeyTypeUnlimited,
		Default:           true,
		Groups:            tags,
		Relay:             cfg.GatewayID.ValueString(),
		AutoAssignGateway: cfg.AutoAssignGateway.ValueBool(),
	}
	return r.client.UpdateEnrollmentKey(ctx, keyValue, req)
}

// decodeDefaultEnrollmentKeyConfig decodes a plan/config value of the
// default_enrollment_key attribute into a concrete
// *DefaultEnrollmentKeyModel, or nil if it's null or unknown (the latter
// happens whenever default_enrollment_key is omitted entirely — it's
// Optional+Computed, so its plan value is "unknown" rather than "null" in
// that case).
func decodeDefaultEnrollmentKeyConfig(ctx context.Context, obj types.Object) (*DefaultEnrollmentKeyModel, diag.Diagnostics) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, nil
	}
	var cfg DefaultEnrollmentKeyModel
	diags := obj.As(ctx, &cfg, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	return &cfg, diags
}

func defaultEnrollmentKeyToObject(ctx context.Context, d *nmclient.EnrollmentKeyDetail) (types.Object, diag.Diagnostics) {
	tagsList, diags := types.ListValueFrom(ctx, types.StringType, d.Tags)
	if diags.HasError() {
		return types.ObjectNull(defaultEnrollmentKeyAttrTypes), diags
	}
	obj, objDiags := types.ObjectValueFrom(ctx, defaultEnrollmentKeyAttrTypes, DefaultEnrollmentKeyModel{
		GatewayID:         stringOrEmpty(d.GatewayID),
		AutoAssignGateway: types.BoolValue(d.AutoAssignGateway),
		Tags:              tagsList,
	})
	diags.Append(objDiags...)
	return obj, diags
}

// applyDefaultEnrollmentKey merges an *nmclient.EnrollmentKeyDetail into a
// NetworkResourceModel's default_enrollment_key(_value|_token) fields.
func applyDefaultEnrollmentKey(ctx context.Context, model *NetworkResourceModel, d *nmclient.EnrollmentKeyDetail) diag.Diagnostics {
	obj, diags := defaultEnrollmentKeyToObject(ctx, d)
	if diags.HasError() {
		return diags
	}
	model.DefaultEnrollmentKey = obj
	model.DefaultValue = types.StringValue(d.Value)
	model.DefaultToken = types.StringValue(d.Token)
	return diags
}

func (r *NetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	n, err := r.client.GetNetwork(ctx, state.Name.ValueString())
	if err != nil {
		var apiErr *nmclient.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading network", err.Error())
		return
	}

	defaultKey, err := r.client.GetDefaultEnrollmentKeyForNetwork(ctx, n.NetID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading network's default enrollment key", err.Error())
		return
	}

	model := networkToModel(n)
	resp.Diagnostics.Append(applyDefaultEnrollmentKey(ctx, &model, defaultKey)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *NetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state NetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	n := &nmclient.Network{
		NetID:               plan.Name.ValueString(),
		AddressRange:        plan.AddressRange.ValueString(),
		AddressRange6:       plan.AddressRange6.ValueString(),
		DefaultKeepAlive:    int(plan.DefaultKeepAlive.ValueInt64()),
		DefaultMTU:          int32(plan.DefaultMTU.ValueInt64()),
		AutoJoin:            plan.AutoJoin.ValueBool(),
		AutoRemove:          plan.AutoRemove.ValueBool(),
		AutoRemoveThreshold: int(plan.AutoRemoveThreshold.ValueInt64()),
		JITEnabled:          plan.JITEnabled.ValueBool(),
	}

	updated, err := r.client.UpdateNetwork(ctx, n)
	if err != nil {
		resp.Diagnostics.AddError("Error updating network", err.Error())
		return
	}

	keyValue := state.DefaultValue.ValueString()
	if keyValue == "" {
		// Guards against state written before this attribute existed.
		dk, err := r.client.GetDefaultEnrollmentKeyForNetwork(ctx, updated.NetID)
		if err != nil {
			resp.Diagnostics.AddError("Error reading network's default enrollment key", err.Error())
			return
		}
		keyValue = dk.Value
	}

	cfg, diags := decodeDefaultEnrollmentKeyConfig(ctx, plan.DefaultEnrollmentKey)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var defaultKey *nmclient.EnrollmentKeyDetail
	if cfg != nil {
		defaultKey, err = r.applyDefaultEnrollmentKeyConfig(ctx, updated.NetID, keyValue, cfg)
	} else {
		defaultKey, err = r.client.GetDefaultEnrollmentKeyForNetwork(ctx, updated.NetID)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error configuring network's default enrollment key", err.Error())
		return
	}

	model := networkToModel(updated)
	resp.Diagnostics.Append(applyDefaultEnrollmentKey(ctx, &model, defaultKey)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *NetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteNetwork(ctx, state.Name.ValueString(), false); err != nil {
		resp.Diagnostics.AddError("Error deleting network", err.Error())
	}
}

func (r *NetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func networkToModel(n *nmclient.Network) NetworkResourceModel {
	return NetworkResourceModel{
		ID:                  types.StringValue(n.ID),
		Name:                types.StringValue(n.NetID),
		AddressRange:        types.StringValue(n.AddressRange),
		AddressRange6:       types.StringValue(n.AddressRange6),
		DefaultKeepAlive:    types.Int64Value(int64(n.DefaultKeepAlive)),
		DefaultMTU:          types.Int64Value(int64(n.DefaultMTU)),
		AutoJoin:            types.BoolValue(n.AutoJoin),
		AutoRemove:          types.BoolValue(n.AutoRemove),
		AutoRemoveThreshold: types.Int64Value(int64(n.AutoRemoveThreshold)),
		JITEnabled:          types.BoolValue(n.JITEnabled),
	}
}
