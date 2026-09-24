package client

import (
	"context"
	"net/url"
	"time"
)

// PendingHost is a host that has registered with an enrollment key for a
// network that doesn't auto-join, and is waiting for an admin to approve it
// on the dashboard (mirrors schema.PendingHost). Until it's approved the
// host isn't a Device (see devices.go) and has no Node in that network.
type PendingHost struct {
	ID          string    `json:"id"`
	HostID      string    `json:"host_id"`
	HostName    string    `json:"host_name"`
	Network     string    `json:"network"`
	RequestedAt time.Time `json:"requested_at"`
}

// ListPendingHosts lists the hosts waiting for approval in a network.
func (c *Client) ListPendingHosts(ctx context.Context, network string) ([]PendingHost, error) {
	var out []PendingHost
	path := "/api/v1/pending_hosts?network=" + url.QueryEscape(network)
	if err := c.requestEnveloped(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PendingNetworksForHost returns the networks in which a host with the given
// name is waiting for approval, or nil if it isn't pending anywhere. There's
// no endpoint to look a pending host up by name, so this checks every
// network.
func (c *Client) PendingNetworksForHost(ctx context.Context, hostName string) ([]string, error) {
	networks, err := c.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}
	var pendingIn []string
	for _, n := range networks {
		pending, err := c.ListPendingHosts(ctx, n.NetID)
		if err != nil {
			return nil, err
		}
		for _, p := range pending {
			if p.HostName == hostName {
				pendingIn = append(pendingIn, n.NetID)
				break
			}
		}
	}
	return pendingIn, nil
}
