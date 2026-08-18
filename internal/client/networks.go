package client

import (
	"context"
	"net/url"
	"time"
)

// Network mirrors the server's schema.Network — the sole wire shape used by
// every network endpoint (create/get/list/update all decode/encode this
// same struct).
type Network struct {
	ID            string `json:"id,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
	NetID         string `json:"netid"`
	AddressRange  string `json:"addressrange,omitempty"`
	AddressRange6 string `json:"addressrange6,omitempty"`

	DefaultKeepAlive int   `json:"defaultkeepalive,omitempty"`
	DefaultMTU       int32 `json:"defaultmtu,omitempty"`

	AutoJoin            bool     `json:"auto_join"`
	AutoRemove          bool     `json:"auto_remove"`
	AutoRemoveTags      []string `json:"auto_remove_tags,omitempty"`
	AutoRemoveThreshold int      `json:"auto_remove_threshold,omitempty"`
	JITEnabled          bool     `json:"jit_enabled"`
	JITUserGroupIDs     []string `json:"jit_user_group_ids,omitempty"`

	VirtualNATPoolIPv4          string `json:"virtual_nat_pool_ipv4,omitempty"`
	VirtualNATSitePrefixLenIPv4 int    `json:"virtual_nat_site_prefixlen_ipv4,omitempty"`

	NodesUpdatedAt time.Time `json:"nodes_updated_at,omitempty"`
	CreatedBy      string    `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// CreateNetwork creates a new network. n.NetID is required; n.ID is
// server-assigned and should be left empty.
func (c *Client) CreateNetwork(ctx context.Context, n *Network) (*Network, error) {
	var out Network
	if err := c.request(ctx, "POST", "/api/networks", n, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetNetwork fetches a network by its netid.
func (c *Client) GetNetwork(ctx context.Context, netID string) (*Network, error) {
	var out Network
	path := "/api/networks/" + url.PathEscape(netID)
	if err := c.request(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListNetworks lists all networks visible to the authenticated tenant.
func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	var out []Network
	if err := c.request(ctx, "GET", "/api/networks", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateNetwork updates a network. Only a subset of fields are persisted by
// the server (default_keep_alive, default_mtu, auto_join, auto_remove,
// auto_remove_tags, auto_remove_threshold, jit_enabled, jit_user_group_ids,
// virtual_nat_pool_ipv4, virtual_nat_site_prefixlen_ipv4) — send the full
// struct (e.g. from a prior Get) with those fields changed; other fields
// such as AddressRange are not editable via this endpoint.
func (c *Client) UpdateNetwork(ctx context.Context, n *Network) (*Network, error) {
	var out Network
	path := "/api/networks/" + url.PathEscape(n.NetID)
	if err := c.request(ctx, "PUT", path, n, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteNetwork deletes a network by netid. force=true removes the network
// even if it still has hosts/nodes attached.
func (c *Client) DeleteNetwork(ctx context.Context, netID string, force bool) error {
	path := "/api/networks/" + url.PathEscape(netID)
	if force {
		path += "?force=true"
	}
	return c.requestEnveloped(ctx, "DELETE", path, nil, nil)
}
