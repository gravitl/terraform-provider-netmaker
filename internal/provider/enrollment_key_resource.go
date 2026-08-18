package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
)

var (
	_ resource.Resource                = &EnrollmentKeyResource{}
	_ resource.ResourceWithConfigure   = &EnrollmentKeyResource{}
	_ resource.ResourceWithImportState = &EnrollmentKeyResource{}
)

func NewEnrollmentKeyResource() resource.Resource {
	return &EnrollmentKeyResource{}
}

// EnrollmentKeyResource implements the netmaker_enrollment_key resource.
type EnrollmentKeyResource struct {
	client *nmclient.Client
}

// EnrollmentKeyResourceModel maps the netmaker_enrollment_key resource
// schema. Value is the resource's identifier: the server's {keyID} path
// parameter (used by update/delete/regenerate-token) is documented as
// "Enrollment Key value", i.e. the same Value returned on create — there
// is no separate internal ID exposed by the API, and no single-key GET
// endpoint, so Read is implemented as a ListEnrollmentKeys + filter.
type EnrollmentKeyResourceModel struct {
	Value             types.String `tfsdk:"value"`
	Token             types.String `tfsdk:"token"`
	Networks          types.List   `tfsdk:"networks"`
	Tags              types.List   `tfsdk:"tags"`
	Type              types.String `tfsdk:"type"`
	Unlimited         types.Bool   `tfsdk:"unlimited"`
	UsesRemaining     types.Int64  `tfsdk:"uses_remaining"`
	ExpirationSeconds types.Int64  `tfsdk:"expiration_unix"`
	GatewayID         types.String `tfsdk:"gateway_id"`
	AutoAssignGateway types.Bool   `tfsdk:"auto_assign_gateway"`
	Default           types.Bool   `tfsdk:"default"`
}

func (r *EnrollmentKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_enrollment_key"
}

func (r *EnrollmentKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Netmaker enrollment key (join token) for one or more networks.",
		Attributes: map[string]schema.Attribute{
			"value": schema.StringAttribute{
				Description: "Server-assigned enrollment key value; also serves as this resource's identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"token": schema.StringAttribute{
				Description: "Base64-encoded join token to pass to `netclient join -t`.",
				Computed:    true,
				Sensitive:   true,
			},
			"networks": schema.ListAttribute{
				Description: "Networks this key allows joining.",
				Required:    true,
				ElementType: types.StringType,
			},
			"tags": schema.ListAttribute{
				Description: "Tags to apply to devices enrolled with this key.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"type": schema.StringAttribute{
				Description: "Key type: time_expiration, uses, or unlimited.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"unlimited": schema.BoolAttribute{
				Description: "Whether this key has unlimited uses. Derived from type = unlimited.",
				Computed:    true,
			},
			"uses_remaining": schema.Int64Attribute{
				Description: "Number of remaining uses. Required when type = uses.",
				Optional:    true,
				Computed:    true,
			},
			"expiration_unix": schema.Int64Attribute{
				Description: "Unix timestamp (seconds) the key expires at. Required when type = time_expiration.",
				Optional:    true,
				Computed:    true,
			},
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
			"default": schema.BoolAttribute{
				Description: "Whether this is a network's auto-created default enrollment key (one is created automatically per network by Netmaker on netmaker_network creation, and deleted along with it — see the README). Not settable here; import an existing default key by its value to manage it (gateway_id, auto_assign_gateway, tags).",
				Computed:    true,
			},
		},
	}
}

func (r *EnrollmentKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnrollmentKeyResource) buildRequest(ctx context.Context, plan EnrollmentKeyResourceModel) (*nmclient.EnrollmentKeyRequest, error) {
	var networks, tags []string
	if err := plan.Networks.ElementsAs(ctx, &networks, false); err != nil {
		return nil, fmt.Errorf("invalid networks: %v", err)
	}
	if !plan.Tags.IsNull() {
		if err := plan.Tags.ElementsAs(ctx, &tags, false); err != nil {
			return nil, fmt.Errorf("invalid tags: %v", err)
		}
	}

	keyType, err := keyTypeFromString(plan.Type.ValueString())
	if err != nil {
		return nil, err
	}

	return &nmclient.EnrollmentKeyRequest{
		Networks: networks,
		// Tags is only used server-side as the key's display name at
		// creation (tags[0]) and is otherwise ignored (including on
		// update) — Groups is what actually persists as the key's
		// readable Tags value, both at creation and on update. Send both
		// so the resource's "tags" attribute behaves as documented.
		Tags:              tags,
		Groups:            tags,
		Type:              keyType,
		Unlimited:         keyType == nmclient.KeyTypeUnlimited,
		UsesRemaining:     int(plan.UsesRemaining.ValueInt64()),
		Expiration:        plan.ExpirationSeconds.ValueInt64(),
		Relay:             plan.GatewayID.ValueString(),
		AutoAssignGateway: plan.AutoAssignGateway.ValueBool(),
	}, nil
}

func keyTypeFromString(s string) (nmclient.KeyType, error) {
	switch s {
	case "time_expiration":
		return nmclient.KeyTypeTimeExpiration, nil
	case "uses":
		return nmclient.KeyTypeUses, nil
	case "unlimited":
		return nmclient.KeyTypeUnlimited, nil
	default:
		return nmclient.KeyTypeUndefined, fmt.Errorf("invalid type %q: must be one of time_expiration, uses, unlimited", s)
	}
}

func keyTypeToString(t nmclient.KeyType) string {
	switch t {
	case nmclient.KeyTypeTimeExpiration:
		return "time_expiration"
	case nmclient.KeyTypeUses:
		return "uses"
	case nmclient.KeyTypeUnlimited:
		return "unlimited"
	default:
		return "undefined"
	}
}

func (r *EnrollmentKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnrollmentKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqBody, err := r.buildRequest(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid enrollment key configuration", err.Error())
		return
	}

	created, err := r.client.CreateEnrollmentKey(ctx, reqBody)
	if err != nil {
		resp.Diagnostics.AddError("Error creating enrollment key", err.Error())
		return
	}

	networksList, diags := types.ListValueFrom(ctx, types.StringType, reqBody.Networks)
	resp.Diagnostics.Append(diags...)
	tagsList, diags := types.ListValueFrom(ctx, types.StringType, reqBody.Tags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := EnrollmentKeyResourceModel{
		Value:             types.StringValue(created.Value),
		Token:             types.StringValue(created.Token),
		Networks:          networksList,
		Tags:              tagsList,
		Type:              types.StringValue(keyTypeToString(created.Type)),
		Unlimited:         types.BoolValue(created.Unlimited),
		UsesRemaining:     types.Int64Value(int64(created.UsesRemaining)),
		ExpirationSeconds: unixOrZero(created.Expiration),
		GatewayID:         types.StringValue(created.Relay),
		AutoAssignGateway: types.BoolValue(created.AutoAssignGateway),
		Default:           types.BoolValue(created.Default),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *EnrollmentKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnrollmentKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys, err := r.client.ListEnrollmentKeys(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading enrollment key", err.Error())
		return
	}
	var found *nmclient.EnrollmentKey
	for i := range keys {
		if keys[i].Value == state.Value.ValueString() {
			found = &keys[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// The list response's Tags field is server-mangled (see EnrollmentKey
	// doc comment: it's overwritten with the key's internal name, not the
	// originally requested tags), and Networks reflects current
	// membership. Preserve the previously known Networks/Tags from state
	// rather than trusting this response for those two fields.
	updated := state
	updated.Token = types.StringValue(found.Token)
	updated.Unlimited = types.BoolValue(found.Unlimited)
	updated.UsesRemaining = types.Int64Value(int64(found.UsesRemaining))
	updated.Type = types.StringValue(keyTypeToString(found.Type))
	updated.ExpirationSeconds = unixOrZero(found.Expiration)
	updated.GatewayID = types.StringValue(found.Relay)
	updated.AutoAssignGateway = types.BoolValue(found.AutoAssignGateway)
	updated.Default = types.BoolValue(found.Default)

	resp.Diagnostics.Append(resp.State.Set(ctx, updated)...)
}

func (r *EnrollmentKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state EnrollmentKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqBody, err := r.buildRequest(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid enrollment key configuration", err.Error())
		return
	}

	updated, err := r.client.UpdateEnrollmentKey(ctx, state.Value.ValueString(), reqBody)
	if err != nil {
		resp.Diagnostics.AddError("Error updating enrollment key", err.Error())
		return
	}

	networksList, diags := types.ListValueFrom(ctx, types.StringType, updated.Networks)
	resp.Diagnostics.Append(diags...)
	tagsList, diags := types.ListValueFrom(ctx, types.StringType, updated.Tags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := EnrollmentKeyResourceModel{
		Value:             types.StringValue(updated.Value),
		Token:             types.StringValue(updated.Token),
		Networks:          networksList,
		Tags:              tagsList,
		Type:              types.StringValue(keyTypeToString(updated.Type)),
		Unlimited:         types.BoolValue(updated.Unlimited),
		UsesRemaining:     types.Int64Value(int64(updated.UsesRemaining)),
		ExpirationSeconds: unixOrZero(updated.Expiration),
		GatewayID:         stringOrEmpty(updated.GatewayID),
		AutoAssignGateway: types.BoolValue(updated.AutoAssignGateway),
		Default:           types.BoolValue(updated.Default),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *EnrollmentKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnrollmentKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteEnrollmentKey(ctx, state.Value.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting enrollment key", err.Error())
	}
}

func (r *EnrollmentKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("value"), req, resp)
}

func unixOrZero(t time.Time) types.Int64 {
	if t.IsZero() {
		return types.Int64Value(0)
	}
	return types.Int64Value(t.Unix())
}

func stringOrEmpty(s *string) types.String {
	if s == nil {
		return types.StringValue("")
	}
	return types.StringValue(*s)
}
