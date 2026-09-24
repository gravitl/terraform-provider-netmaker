package client

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// ExtClientRequest is the request body for creating or updating an ext
// client (mirrors models.CustomExtClient).
type ExtClientRequest struct {
	ClientID        string   `json:"clientid,omitempty"`
	PublicKey       string   `json:"publickey,omitempty"`
	DNS             string   `json:"dns,omitempty"`
	ExtraAllowedIPs []string `json:"extraallowedips,omitempty"`
	// Enabled is always sent: on update the server reads a missing field as
	// false, which would disable the client.
	Enabled        bool                `json:"enabled"`
	PostUp         string              `json:"postup,omitempty"`
	PostDown       string              `json:"postdown,omitempty"`
	Tags           map[string]struct{} `json:"tags,omitempty"`
	DeviceID       string              `json:"device_id,omitempty"`
	DeviceName     string              `json:"device_name,omitempty"`
	PublicEndpoint string              `json:"public_endpoint,omitempty"`
	OS             string              `json:"os,omitempty"`
}

// ExtClient is a remote-access client attached to an ingress gateway node
// (mirrors schema.ExtClient / models.ExtClient — the same struct on the
// server). Unlike a Device, an ExtClient isn't a full netclient host; it's
// typically a mobile/desktop remote-access-client config.
type ExtClient struct {
	ClientID   string `json:"clientid"`
	PrivateKey string `json:"privatekey"`
	PublicKey  string `json:"publickey"`
	Network    string `json:"network"`
	DNS        string `json:"dns"`
	Address    string `json:"address"`
	Address6   string `json:"address6"`

	ExtraAllowedIPs []string `json:"extraallowedips"`
	AllowedIPs      []string `json:"allowed_ips"`

	IngressGatewayID       string `json:"ingressgatewayid"`
	IngressGatewayEndpoint string `json:"ingressgatewayendpoint"`

	Enabled bool   `json:"enabled"`
	OwnerID string `json:"ownerid"`

	PostUp   string `json:"postup"`
	PostDown string `json:"postdown"`

	Tags map[string]struct{} `json:"tags"`

	OS             string `json:"os"`
	DeviceID       string `json:"device_id"`
	DeviceName     string `json:"device_name"`
	PublicEndpoint string `json:"public_endpoint"`

	Status string `json:"status"`

	LastModified int64 `json:"lastmodified"`
}

// ListExtClients lists all ext clients for the tenant, across all networks.
func (c *Client) ListExtClients(ctx context.Context) ([]ExtClient, error) {
	var out []ExtClient
	if err := c.request(ctx, "GET", "/api/extclients", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListExtClientsByNetwork lists all ext clients in a single network.
func (c *Client) ListExtClientsByNetwork(ctx context.Context, network string) ([]ExtClient, error) {
	var out []ExtClient
	path := "/api/extclients/" + url.PathEscape(network)
	if err := c.request(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetExtClient fetches a single ext client by network and client ID. Unlike
// the list endpoints, this includes the client's private key.
func (c *Client) GetExtClient(ctx context.Context, network, clientID string) (*ExtClient, error) {
	var out ExtClient
	path := "/api/extclients/" + url.PathEscape(network) + "/" + url.PathEscape(clientID)
	if err := c.request(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetExtClientConfigFile fetches the server-rendered wg-quick config text
// for an ext client (GET .../{clientid}/file) — includes the resolved
// Endpoint/AllowedIPs/PostUp/PostDown (e.g. internet-egress vs plain
// network CIDR), ready to write to disk and bring up with wg-quick. Unlike
// every other method on this client, the response is raw text, not JSON.
func (c *Client) GetExtClientConfigFile(ctx context.Context, network, clientID string) (string, error) {
	path := "/api/extclients/" + url.PathEscape(network) + "/" + url.PathEscape(clientID) + "/file"
	data, err := c.rawRequest(ctx, "GET", path, nil)
	if err != nil {
		return "", err
	}
	conf := string(data)
	// When the gateway host has no endpoint IP, the server doesn't fail — it
	// renders the config with a literal "<nil>" (e.g. `Endpoint =
	// [<nil>]:51821`), which wg-quick then rejects with a confusing DNS
	// error. Catch it here instead.
	if strings.Contains(conf, "<nil>") {
		return "", ErrNoGatewayEndpoint
	}
	return conf, nil
}

// ErrNoGatewayEndpoint means the ingress gateway's host has no endpoint IP
// (its netclient hasn't reported a public IP), so the server can't render a
// usable ext client config for it.
var ErrNoGatewayEndpoint = errors.New("nmclient: the ingress gateway's host has no endpoint IP, so the server rendered an ext client config with an empty endpoint (\"<nil>\")")

// GetGatewayEndpoint returns the endpoint IP currently known for the host
// behind an ingress gateway node — its IPv4 endpoint if set, otherwise its
// IPv6 one — or "" if the host hasn't reported one yet. The endpoint is
// reported by the host's netclient daemon after it detects its public IP,
// so it can lag a freshly created gateway node (or never appear, e.g. for a
// host still pending approval).
func (c *Client) GetGatewayEndpoint(ctx context.Context, network, nodeID string) (string, error) {
	node, err := c.GetNode(ctx, network, nodeID)
	if err != nil {
		return "", err
	}
	device, err := c.GetDevice(ctx, node.HostID)
	if err != nil {
		return "", err
	}
	for _, ip := range []string{device.EndpointIP, device.EndpointIPv6} {
		if ip != "" && !strings.Contains(ip, "nil") {
			return ip, nil
		}
	}
	return "", nil
}

// CreateExtClient creates a new ext client attached to the given ingress
// gateway node.
//
// The server's create endpoint ignores two fields of req: it always creates
// the client enabled and always with no tags. Those are only honored by the
// update endpoint, so when req asks for either, this follows the create with
// an update and returns the updated client.
func (c *Client) CreateExtClient(ctx context.Context, network, ingressNodeID string, req *ExtClientRequest) (*ExtClient, error) {
	var out ExtClient
	path := "/api/extclients/" + url.PathEscape(network) + "/" + url.PathEscape(ingressNodeID)
	if err := c.request(ctx, "POST", path, req, &out); err != nil {
		return nil, err
	}
	if !req.Enabled || len(req.Tags) > 0 {
		return c.UpdateExtClient(ctx, network, out.ClientID, req)
	}
	return &out, nil
}

// UpdateExtClient updates an existing ext client.
func (c *Client) UpdateExtClient(ctx context.Context, network, clientID string, req *ExtClientRequest) (*ExtClient, error) {
	var out ExtClient
	path := "/api/extclients/" + url.PathEscape(network) + "/" + url.PathEscape(clientID)
	if err := c.request(ctx, "PUT", path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteExtClient deletes an ext client.
func (c *Client) DeleteExtClient(ctx context.Context, network, clientID string) error {
	path := "/api/extclients/" + url.PathEscape(network) + "/" + url.PathEscape(clientID)
	return c.requestEnveloped(ctx, "DELETE", path, nil, nil)
}
