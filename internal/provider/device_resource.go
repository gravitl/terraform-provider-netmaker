package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	_ resource.Resource               = &DeviceResource{}
	_ resource.ResourceWithConfigure  = &DeviceResource{}
	_ resource.ResourceWithModifyPlan = &DeviceResource{}
)

func NewDeviceResource() resource.Resource {
	return &DeviceResource{}
}

// DeviceResource implements the netmaker_device resource: provisions a
// fresh machine by installing a pinned netclient version and joining it to
// whatever network(s) the given enrollment key covers. There is no
// adoption path — this resource always provisions a new host; an existing
// host can only be inspected via the netmaker_device data source.
type DeviceResource struct {
	client *nmclient.Client
}

// DeviceResourceModel maps the netmaker_device resource schema.
type DeviceResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	NetclientVersion types.String `tfsdk:"netclient_version"`
	AutoUpdate       types.Bool   `tfsdk:"auto_update"`
	OS               types.String `tfsdk:"os"`
	Nodes            types.List   `tfsdk:"nodes"`
	Deploy           *DeployModel `tfsdk:"deploy"`
}

// resolveVersioning validates and resolves netclient_version/auto_update:
// exactly one of a pinned version or auto_update = true must apply.
// Pinning a version always forces auto_update off (a device can't
// self-update away from the version you asked for); leaving the version
// unset only makes sense if netclient is going to keep itself current.
func (m DeviceResourceModel) resolveVersioning() (version string, autoUpdate bool, err error) {
	version = m.NetclientVersion.ValueString()
	versionSet := !m.NetclientVersion.IsNull() && !m.NetclientVersion.IsUnknown() && version != ""
	autoUpdateSet := !m.AutoUpdate.IsNull() && !m.AutoUpdate.IsUnknown()
	autoUpdateTrue := autoUpdateSet && m.AutoUpdate.ValueBool()

	switch {
	case versionSet && autoUpdateTrue:
		return "", false, fmt.Errorf("netclient_version and auto_update = true are mutually exclusive: pinning a version means netclient can't also self-update")
	case versionSet:
		return version, false, nil
	case autoUpdateTrue:
		return "", true, nil
	default:
		return "", false, fmt.Errorf("one of netclient_version or auto_update = true must be set")
	}
}

func (r *DeviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_device"
}

func (r *DeviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a fresh machine as a Netmaker device (Host): installs a pinned netclient version and joins it to the network(s) covered by the given enrollment key. Always provisions a new host — does not adopt an existing one.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Server-assigned device (host) ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Device name, passed to `netclient join -o` and used to locate the resulting device after joining. Changing this forces recreation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"netclient_version": schema.StringAttribute{
				Description: "netclient release version to install, e.g. \"v1.2.0\" (see https://github.com/gravitl/netclient/releases). Mutually exclusive with auto_update = true — exactly one of the two must be set. Changing this swaps the installed version in place (`netclient use`) rather than reinstalling/rejoining.",
				Optional:    true,
			},
			"auto_update": schema.BoolAttribute{
				Description: "Whether netclient keeps itself updated to the latest release instead of a pinned version. Forced to false whenever netclient_version is set. Exactly one of netclient_version or auto_update = true must be set. Changing this only flips the device's server-side auto_update flag — no reinstall.",
				Optional:    true,
				Computed:    true,
			},
			"os": schema.StringAttribute{
				Description: "Detected target OS: linux, darwin, or windows.",
				Computed:    true,
			},
			"nodes": schema.ListAttribute{
				Description: "IDs of the Node records created by joining (one per network the enrollment key covers).",
				Computed:    true,
				ElementType: types.StringType,
			},
			"deploy": deploySchema(),
		},
	}
}

func (r *DeviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan validates the deploy.mode block early (at plan time) rather
// than failing mid-apply after already dialing out.
func (r *DeviceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return // destroy
	}
	var plan DeviceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Deploy != nil && plan.Deploy.Mode != nil {
		if err := plan.Deploy.Mode.validate(); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("deploy").AtName("mode"), "Invalid deploy configuration", err.Error())
		}
	}
	if _, _, err := plan.resolveVersioning(); err != nil {
		resp.Diagnostics.AddError("Invalid netclient_version/auto_update configuration", err.Error())
	}
}

func (r *DeviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DeviceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	version, autoUpdate, err := plan.resolveVersioning()
	if err != nil {
		resp.Diagnostics.AddError("Invalid netclient_version/auto_update configuration", err.Error())
		return
	}

	connCfg, err := plan.Deploy.Mode.toProvisionerConfig()
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

	name := plan.Name.ValueString()
	if err := provisioner.InstallAndJoin(conn, info, version, plan.Deploy.Token.ValueString(), name); err != nil {
		resp.Diagnostics.AddError("Error installing/joining netclient", err.Error())
		return
	}

	device, err := r.findDeviceByName(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Error locating device after join", err.Error())
		return
	}
	if device == nil {
		// A network with auto_join disabled doesn't add the device when it
		// joins; it's held as a pending host until an admin approves it on
		// the dashboard.
		if pendingIn, err := r.client.PendingNetworksForHost(ctx, name); err == nil && len(pendingIn) > 0 {
			resp.Diagnostics.AddError(
				"Device is pending approval",
				fmt.Sprintf("netclient joined, but the device %q wasn't created: network(s) %s have auto_join disabled, so it's waiting for an admin to approve it on the Netmaker dashboard. Approve it there and apply again (re-running the install/join on the machine is safe), or set auto_join = true on the network(s) so devices are added immediately.",
					name, strings.Join(pendingIn, ", ")),
			)
			return
		}
		resp.Diagnostics.AddError("Device not found after join", fmt.Sprintf("no device named %q appeared after netclient join; check the target machine's netclient logs", name))
		return
	}

	if device.AutoUpdate != autoUpdate {
		device.AutoUpdate = autoUpdate
		device, err = r.client.UpdateDevice(ctx, device.ID, device)
		if err != nil {
			resp.Diagnostics.AddError("Error setting device auto_update", err.Error())
			return
		}
	}

	model, diags := deviceResourceToModel(ctx, device, string(info.OS), plan.Deploy)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	model.NetclientVersion = plan.NetclientVersion
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

// findDeviceByName polls ListDevices for a device with the given name,
// tolerating brief propagation delay right after join.
func (r *DeviceResource) findDeviceByName(ctx context.Context, name string) (*nmclient.Device, error) {
	const attempts = 10
	for i := 0; i < attempts; i++ {
		devices, err := r.client.ListDevices(ctx)
		if err != nil {
			return nil, err
		}
		for j := range devices {
			if devices[j].Name == name {
				return &devices[j], nil
			}
		}
		if i < attempts-1 {
			time.Sleep(2 * time.Second)
		}
	}
	return nil, nil
}

func (r *DeviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DeviceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	device, err := r.client.GetDevice(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *nmclient.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading device", err.Error())
		return
	}

	model, diags := deviceResourceToModel(ctx, device, state.OS.ValueString(), state.Deploy)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Identity fields not returned by GetDevice are preserved from state.
	model.NetclientVersion = state.NetclientVersion
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

// Update runs for changes to netclient_version/auto_update/deploy.mode
// (name and deploy.token are RequiresReplace — everything else can be
// reconciled in place). A version change swaps the installed binary via
// `netclient use` instead of reinstalling/rejoining; an auto_update change
// is just a server-side flag flip via UpdateDevice. deploy.mode changes on
// their own need no remote action — it's only how Terraform reaches the
// machine, not the machine's state.
func (r *DeviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state DeviceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	newVersion, newAutoUpdate, err := plan.resolveVersioning()
	if err != nil {
		resp.Diagnostics.AddError("Invalid netclient_version/auto_update configuration", err.Error())
		return
	}
	oldVersion, _, err := state.resolveVersioning()
	if err != nil {
		// State predates this validation (or was left inconsistent by a
		// prior bug) — don't block the update on it, just treat as unset.
		oldVersion = ""
	}

	if newVersion != "" && newVersion != oldVersion {
		connCfg, err := plan.Deploy.Mode.toProvisionerConfig()
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
		if err := provisioner.UseVersion(conn, info, newVersion); err != nil {
			resp.Diagnostics.AddError("Error switching netclient version", err.Error())
			return
		}
	}

	device, err := r.client.GetDevice(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading device", err.Error())
		return
	}
	if device.AutoUpdate != newAutoUpdate {
		device.AutoUpdate = newAutoUpdate
		device, err = r.client.UpdateDevice(ctx, device.ID, device)
		if err != nil {
			resp.Diagnostics.AddError("Error setting device auto_update", err.Error())
			return
		}
	}

	model, diags := deviceResourceToModel(ctx, device, state.OS.ValueString(), plan.Deploy)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	model.NetclientVersion = plan.NetclientVersion
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *DeviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DeviceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Deploy == nil || state.Deploy.Mode == nil {
		resp.Diagnostics.AddError("Missing deploy configuration in state", "cannot uninstall netclient without connection details")
		return
	}

	connCfg, err := state.Deploy.Mode.toProvisionerConfig()
	if err != nil {
		resp.Diagnostics.AddError("Invalid deploy configuration", err.Error())
		return
	}
	conn, err := provisioner.Connect(connCfg)
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Could not reach device to uninstall netclient",
			"The machine may already be terminated. Removing from Terraform state without uninstalling: "+err.Error(),
		)
		return
	}
	defer conn.Close()

	info, err := provisioner.Detect(conn)
	if err != nil {
		resp.Diagnostics.AddError("Error detecting target OS", err.Error())
		return
	}

	if err := provisioner.Uninstall(conn, info); err != nil {
		resp.Diagnostics.AddError("Error uninstalling netclient", err.Error())
	}
}

func deviceResourceToModel(ctx context.Context, d *nmclient.Device, os string, deploy *DeployModel) (DeviceResourceModel, diag.Diagnostics) {
	nodes, diags := types.ListValueFrom(ctx, types.StringType, d.Nodes)
	return DeviceResourceModel{
		ID:         types.StringValue(d.ID),
		Name:       types.StringValue(d.Name),
		AutoUpdate: types.BoolValue(d.AutoUpdate),
		OS:         types.StringValue(os),
		Nodes:      nodes,
		Deploy:     deploy,
	}, diags
}
