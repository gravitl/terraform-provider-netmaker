package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
)

// Ensure NetmakerProvider satisfies the provider.Provider interface.
var _ provider.Provider = &NetmakerProvider{}

// NetmakerProvider is the Terraform provider implementation.
type NetmakerProvider struct {
	// version is set to the provider version at build time.
	version string
}

// NetmakerProviderModel describes the provider configuration block.
type NetmakerProviderModel struct {
	APIURL   types.String `tfsdk:"api_url"`
	APIToken types.String `tfsdk:"api_token"`
	TenantID types.String `tfsdk:"tenant_id"`
}

// providerData is stashed on resp.ResourceData / resp.DataSourceData so
// resources and data sources can reach the API client.
type providerData struct {
	client *nmclient.Client
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &NetmakerProvider{version: version}
	}
}

func (p *NetmakerProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "netmaker"
	resp.Version = p.version
}

func (p *NetmakerProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Netmaker networks, enrollment keys, tags, devices, nodes, and ext clients. Requires Netmaker " + minServerVersion + " or newer.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Description: "Base URL of the Netmaker server, e.g. https://netmaker.example.com. May also be set via the NETMAKER_API_URL environment variable.",
				Optional:    true,
			},
			"api_token": schema.StringAttribute{
				Description: "Netmaker user access token (PAT) to authenticate with. Optional: omit to call only Netmaker's public endpoints. May also be set via the NETMAKER_API_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"tenant_id": schema.StringAttribute{
				Description: "Tenant ID to scope requests to. Only required on multi-tenant Netmaker deployments. May also be set via the NETMAKER_TENANT_ID environment variable.",
				Optional:    true,
			},
		},
	}
}

func (p *NetmakerProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config NetmakerProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiURL := valueOrEnv(config.APIURL, "NETMAKER_API_URL")
	apiToken := valueOrEnv(config.APIToken, "NETMAKER_API_TOKEN")
	tenantID := valueOrEnv(config.TenantID, "NETMAKER_TENANT_ID")

	if apiURL == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"), "Missing Netmaker API URL",
			"Set the api_url provider attribute or the NETMAKER_API_URL environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// api_token is optional: without it, the provider can still call
	// Netmaker's public endpoints, but any authenticated resource/data
	// source call will fail server-side.
	var opts []nmclient.Option
	if apiToken != "" {
		opts = append(opts, nmclient.WithToken(nmclient.StaticTokenProvider(apiToken)))
	}
	if tenantID != "" {
		opts = append(opts, nmclient.WithTenantID(tenantID))
	}
	client := nmclient.New(apiURL, opts...)

	// Confirm connectivity and check the server version. An unreachable
	// server is only a warning (e.g. GetServerInfo itself may be disabled
	// on some deployments), but a reachable server running an
	// incompatible version is a hard failure — this provider relies on
	// server APIs (e.g. netmaker_tag, netmaker_node.is_ingress_gateway)
	// that don't exist before minServerVersion.
	if info, err := client.GetServerInfo(ctx); err != nil {
		resp.Diagnostics.AddWarning(
			"Could not reach Netmaker server",
			"GetServerInfo failed, continuing without a version check: "+err.Error(),
		)
	} else {
		tflog.Info(ctx, "connected to Netmaker server", map[string]any{"version": info.Version})
		if err := checkServerVersion(info.Version); err != nil {
			resp.Diagnostics.AddError("Unsupported Netmaker server version", err.Error())
			return
		}
	}

	data := &providerData{client: client}
	resp.ResourceData = data
	resp.DataSourceData = data
}

func (p *NetmakerProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNetworkResource,
		NewEnrollmentKeyResource,
		NewNodeResource,
		NewDeviceResource,
		NewExtClientResource,
		NewTagResource,
	}
}

func (p *NetmakerProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewNetworkDataSource,
		NewNetworksDataSource,
		NewDeviceDataSource,
		NewDevicesDataSource,
		NewNodeDataSource,
		NewNodesDataSource,
	}
}

func valueOrEnv(v types.String, envVar string) string {
	if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		return v.ValueString()
	}
	return os.Getenv(envVar)
}
