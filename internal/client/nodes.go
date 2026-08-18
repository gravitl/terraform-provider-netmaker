package client

import (
	"context"
	"fmt"
	"net/url"
)

// Node is a device's per-network membership record (mirrors
// models.ApiNode). A Node is created as a side effect of a Device joining a
// network (see AddDeviceToNetwork, or a device's own `netclient join`) —
// there is no direct "create node" API, so this client only exposes read
// and delete operations.
type Node struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	HostID   string `json:"hostid"`

	Network      string `json:"network"`
	NetworkRange string `json:"networkrange"`

	Address      string   `json:"address"`
	Address6     string   `json:"address6"`
	LocalAddress string   `json:"localaddress"`
	AllowedIPs   []string `json:"allowedips"`

	LastModified int64 `json:"lastmodified"`
	LastCheckIn  int64 `json:"lastcheckin"`

	Connected     bool `json:"connected"`
	PendingDelete bool `json:"pendingdelete"`

	IsEgressGateway   bool   `json:"isegressgateway"`
	IsIngressGateway  bool   `json:"isingressgateway"`
	IsRelay           bool   `json:"isrelay"`
	IsRelayed         bool   `json:"isrelayed"`
	RelayedBy         string `json:"relayedby"`
	IsGw              bool   `json:"is_gw"`
	IsInternetGateway bool   `json:"isinternetgateway"`

	Tags   map[string]struct{} `json:"tags"`
	Status string              `json:"status"`

	Server   string `json:"server"`
	Metadata string `json:"metadata"`
}

// ListNodes lists all nodes for the tenant, across all networks.
func (c *Client) ListNodes(ctx context.Context) ([]Node, error) {
	var out []Node
	if err := c.request(ctx, "GET", "/api/nodes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListNodesByNetwork lists all nodes in a single network.
func (c *Client) ListNodesByNetwork(ctx context.Context, network string) ([]Node, error) {
	var out []Node
	path := "/api/nodes/" + url.PathEscape(network)
	if err := c.request(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetNode fetches a single node by network and node ID.
//
// The server's single-node-get endpoint returns a differently-shaped,
// internal representation (models.NodeGet, wrapping the node alongside its
// host, peers, and server config) rather than the models.ApiNode shape used
// everywhere else. To keep this client's Node type consistent across all
// operations, GetNode is implemented as a lookup against
// ListNodesByNetwork rather than calling that endpoint directly.
func (c *Client) GetNode(ctx context.Context, network, nodeID string) (*Node, error) {
	nodes, err := c.ListNodesByNetwork(ctx, network)
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		if nodes[i].ID == nodeID {
			return &nodes[i], nil
		}
	}
	return nil, fmt.Errorf("nmclient: node %q not found in network %q", nodeID, network)
}

// DeleteNode removes a node from a network (the device leaves that
// network). force=true deletes it even if it has unresolved dependents.
func (c *Client) DeleteNode(ctx context.Context, network, nodeID string, force bool) error {
	path := "/api/nodes/" + url.PathEscape(network) + "/" + url.PathEscape(nodeID)
	if force {
		path += "?force=true"
	}
	return c.requestEnveloped(ctx, "DELETE", path, nil, nil)
}
