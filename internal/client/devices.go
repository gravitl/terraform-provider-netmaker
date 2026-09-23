package client

import (
	"context"
	"net/url"
)

// DeviceInterface describes one network interface reported by a device
// (mirrors models.ApiIface).
type DeviceInterface struct {
	Name          string `json:"name"`
	AddressString string `json:"addressString"`
}

// Device is a machine running netclient (mirrors models.ApiHost — the
// server's "Host" entity). A Device is network-independent; joining a
// network produces a Node (see nodes.go) that references this Device's ID
// via HostID.
type Device struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	OS            string `json:"os"`
	OSFamily      string `json:"os_family"`
	OSVersion     string `json:"os_version"`
	KernelVersion string `json:"kernel_version"`

	PublicKey  string `json:"publickey"`
	MacAddress string `json:"macaddress"`

	EndpointIP   string `json:"endpointip"`
	EndpointIPv6 string `json:"endpointipv6"`
	ListenPort   int    `json:"listenport"`
	MTU          int    `json:"mtu"`

	Interfaces       []DeviceInterface `json:"interfaces"`
	DefaultInterface string            `json:"defaultinterface"`

	Nodes []string `json:"nodes"`

	IsStaticPort        bool `json:"isstaticport"`
	IsStatic            bool `json:"isstatic"`
	IsDefault           bool `json:"isdefault"`
	PersistentKeepalive int  `json:"persistentkeepalive"` // seconds
	AutoUpdate          bool `json:"autoupdate"`

	DNS            string `json:"dns"`
	EnableFlowLogs bool   `json:"enable_flow_logs"`
	Location       string `json:"location"`
	CountryCode    string `json:"country_code"`

	SerialNumber string `json:"serial_number"`
	HardwareUUID string `json:"hardware_uuid"`
}

// ListDevices lists all devices (hosts) for the tenant.
func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	var out []Device
	if err := c.request(ctx, "GET", "/api/hosts", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetDevice fetches a single device by ID.
func (c *Client) GetDevice(ctx context.Context, deviceID string) (*Device, error) {
	var out Device
	path := "/api/hosts/" + url.PathEscape(deviceID)
	if err := c.requestEnveloped(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateDevice updates a device's editable fields.
func (c *Client) UpdateDevice(ctx context.Context, deviceID string, d *Device) (*Device, error) {
	var out Device
	path := "/api/hosts/" + url.PathEscape(deviceID)
	if err := c.request(ctx, "PUT", path, d, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteDevice deletes a device by ID. This also removes it from any
// networks it had joined.
func (c *Client) DeleteDevice(ctx context.Context, deviceID string) error {
	path := "/api/v1/ui/hosts/" + url.PathEscape(deviceID)
	return c.request(ctx, "DELETE", path, nil, nil)
}

// AddDeviceToNetwork joins a device to a network, creating a Node. Use an
// enrollment key from the device side (netclient join) for normal
// enrollment; this endpoint is for attaching an already-registered device
// to an additional network from the control plane.
func (c *Client) AddDeviceToNetwork(ctx context.Context, deviceID, network string) error {
	path := "/api/hosts/" + url.PathEscape(deviceID) + "/networks/" + url.PathEscape(network)
	return c.request(ctx, "POST", path, nil, nil)
}

// RemoveDeviceFromNetwork removes a device from a network, deleting its
// Node. force=true removes it even if the node has unresolved dependents
// (e.g. it's a relay/gateway other nodes depend on).
func (c *Client) RemoveDeviceFromNetwork(ctx context.Context, deviceID, network string, force bool) error {
	path := "/api/hosts/" + url.PathEscape(deviceID) + "/networks/" + url.PathEscape(network)
	if force {
		path += "?force=true"
	}
	return c.request(ctx, "DELETE", path, nil, nil)
}
