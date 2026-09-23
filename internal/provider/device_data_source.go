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
	_ datasource.DataSource              = &DeviceDataSource{}
	_ datasource.DataSourceWithConfigure = &DeviceDataSource{}
)

func NewDeviceDataSource() datasource.DataSource {
	return &DeviceDataSource{}
}

// DeviceDataSource implements the netmaker_device data source.
type DeviceDataSource struct {
	client *nmclient.Client
}

// DeviceModel maps a Device (Host) to Terraform-facing attributes, shared
// between the netmaker_device and netmaker_devices data sources.
type DeviceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Version      types.String `tfsdk:"version"`
	OS           types.String `tfsdk:"os"`
	PublicKey    types.String `tfsdk:"public_key"`
	EndpointIP   types.String `tfsdk:"endpoint_ip"`
	EndpointIPv6 types.String `tfsdk:"endpoint_ipv6"`
	ListenPort   types.Int64  `tfsdk:"listen_port"`
	Nodes        types.List   `tfsdk:"nodes"`
}

func deviceModelSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id":            schema.StringAttribute{Computed: true},
		"name":          schema.StringAttribute{Computed: true},
		"version":       schema.StringAttribute{Computed: true},
		"os":            schema.StringAttribute{Computed: true},
		"public_key":    schema.StringAttribute{Computed: true},
		"endpoint_ip":   schema.StringAttribute{Computed: true},
		"endpoint_ipv6": schema.StringAttribute{Computed: true},
		"listen_port":   schema.Int64Attribute{Computed: true},
		"nodes": schema.ListAttribute{
			Computed:    true,
			ElementType: types.StringType,
			Description: "IDs of this device's per-network Node records.",
		},
	}
}

func deviceToModel(ctx context.Context, d *nmclient.Device) (DeviceModel, error) {
	nodes, diags := types.ListValueFrom(ctx, types.StringType, d.Nodes)
	if diags.HasError() {
		return DeviceModel{}, fmt.Errorf("converting nodes list: %v", diags)
	}
	return DeviceModel{
		ID:           types.StringValue(d.ID),
		Name:         types.StringValue(d.Name),
		Version:      types.StringValue(d.Version),
		OS:           types.StringValue(d.OS),
		PublicKey:    types.StringValue(d.PublicKey),
		EndpointIP:   types.StringValue(d.EndpointIP),
		EndpointIPv6: types.StringValue(d.EndpointIPv6),
		ListenPort:   types.Int64Value(int64(d.ListenPort)),
		Nodes:        nodes,
	}, nil
}

func (d *DeviceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_device"
}

func (d *DeviceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := deviceModelSchema()
	attrs["id"] = schema.StringAttribute{Required: true, Description: "Device (host) ID to look up."}
	resp.Schema = schema.Schema{
		Description: "Looks up an existing Netmaker device (host) by ID.",
		Attributes:  attrs,
	}
}

func (d *DeviceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DeviceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model DeviceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	device, err := d.client.GetDevice(ctx, model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading device", err.Error())
		return
	}

	out, err := deviceToModel(ctx, device)
	if err != nil {
		resp.Diagnostics.AddError("Error converting device", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, out)...)
}
