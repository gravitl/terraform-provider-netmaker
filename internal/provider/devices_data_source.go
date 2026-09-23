package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
)

var (
	_ datasource.DataSource              = &DevicesDataSource{}
	_ datasource.DataSourceWithConfigure = &DevicesDataSource{}
)

func NewDevicesDataSource() datasource.DataSource {
	return &DevicesDataSource{}
}

// DevicesDataSource implements the netmaker_devices data source, listing
// every device (host) visible to the tenant.
type DevicesDataSource struct {
	client *nmclient.Client
}

type DevicesDataSourceModel struct {
	Devices []DeviceModel `tfsdk:"devices"`
}

func (d *DevicesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_devices"
}

func (d *DevicesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all Netmaker devices (hosts) visible to the tenant.",
		Attributes: map[string]schema.Attribute{
			"devices": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: deviceModelSchema(),
				},
			},
		},
	}
}

func (d *DevicesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DevicesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	devices, err := d.client.ListDevices(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing devices", err.Error())
		return
	}

	model := DevicesDataSourceModel{Devices: make([]DeviceModel, 0, len(devices))}
	for i := range devices {
		m, err := deviceToModel(ctx, &devices[i])
		if err != nil {
			resp.Diagnostics.AddError("Error converting device", err.Error())
			return
		}
		model.Devices = append(model.Devices, m)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
