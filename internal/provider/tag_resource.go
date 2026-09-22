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
	_ resource.Resource              = &TagResource{}
	_ resource.ResourceWithConfigure = &TagResource{}
)

func NewTagResource() resource.Resource {
	return &TagResource{}
}

// TagResource implements the netmaker_tag resource. A tag is a
// network-scoped identifier (id = "<network>.<name>") that other resources
// — netmaker_enrollment_key's tags, ACLs, posture checks, egress, ... —
// reference by name; it has no lifecycle of its own tied to any of them.
// Netmaker's own API doesn't validate that a referenced tag exists (e.g.
// an enrollment key can be created with a Groups entry for a tag that was
// never created), so this resource exists to let Terraform create the tag
// first and let netmaker_enrollment_key fail loudly instead of silently
// persisting a broken reference — see that resource's doc comment.
type TagResource struct {
	client *nmclient.Client
}

// TagResourceModel maps the netmaker_tag resource schema.
type TagResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Network   types.String `tfsdk:"network"`
	Name      types.String `tfsdk:"name"`
	ColorCode types.String `tfsdk:"color_code"`
}

func (r *TagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (r *TagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Netmaker tag: a network-scoped identifier that other resources (netmaker_enrollment_key's tags, ACLs, posture checks, egress) reference by name. Create this before referencing its name elsewhere — Netmaker doesn't auto-create tags, and referencing one that doesn't exist yet leaves a broken reference rather than failing (see netmaker_enrollment_key's doc comment). The one exception is netmaker_network's default_enrollment_key.tags, which auto-creates any tag that doesn't already exist, since that key is created as a side effect of network creation, before a netmaker_tag resource for it could exist.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Tag ID: \"<network>.<name>\".",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"network": schema.StringAttribute{
				Description: "Network this tag belongs to. Changing this forces recreation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Tag name, 3-32 characters (letters, digits, spaces, hyphens). Changing this forces recreation — a tag's ID is derived from its name, so renaming is a create/delete, not an update.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"color_code": schema.StringAttribute{
				Description: "Display color for this tag (e.g. \"#4287f5\").",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *TagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateTag(ctx, plan.Network.ValueString(), plan.Name.ValueString(), plan.ColorCode.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating tag", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, tagToModel(created))...)
}

// findTag looks up a tag by network+name via ListTags — the API has no
// single-tag GET endpoint.
func (r *TagResource) findTag(ctx context.Context, network, name string) (*nmclient.Tag, error) {
	tags, err := r.client.ListTags(ctx, network)
	if err != nil {
		return nil, err
	}
	for i := range tags {
		if tags[i].TagName == name {
			return &tags[i], nil
		}
	}
	return nil, nil
}

func (r *TagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tag, err := r.findTag(ctx, state.Network.ValueString(), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading tag", err.Error())
		return
	}
	if tag == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, tagToModel(tag))...)
}

func (r *TagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// network/name are RequiresReplace; only color_code can change here.
	var plan, state TagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateTag(ctx, state.ID.ValueString(), plan.ColorCode.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error updating tag", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, tagToModel(updated))...)
}

func (r *TagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteTag(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting tag", err.Error())
	}
}

func tagToModel(t *nmclient.Tag) TagResourceModel {
	return TagResourceModel{
		ID:        types.StringValue(t.ID),
		Network:   types.StringValue(t.Network),
		Name:      types.StringValue(t.TagName),
		ColorCode: types.StringValue(t.ColorCode),
	}
}
