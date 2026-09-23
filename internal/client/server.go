package client

import "context"

// ServerInfo is returned by GetServerInfo (mirrors models.ServerConfig).
//
// The server's ServerConfig struct has no json tags on most fields (only
// yaml tags), so encoding/json falls back to the exported Go field names —
// meaning most JSON keys here are capitalized, which is unusual but
// intentional and matches the server's actual wire format.
type ServerInfo struct {
	TenantID string `json:"TenantID"`
	Server   string `json:"Server"`
	API      string `json:"API"`
	APIHost  string `json:"APIHost"`
	APIPort  string `json:"APIPort"`
	Version  string `json:"Version"`
	IsPro    bool   `json:"Is_EE"`
}

// GetServerInfo fetches basic server information, including Version — use
// this at provider configure-time to check compatibility with the
// connected Netmaker server.
func (c *Client) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	var out ServerInfo
	if err := c.request(ctx, "GET", "/api/server/getserverinfo", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
