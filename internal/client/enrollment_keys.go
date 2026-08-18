package client

import (
	"context"
	"net/url"
	"time"
)

// KeyType matches models.EnrollmentKey's KeyType enum.
type KeyType int

const (
	KeyTypeUndefined      KeyType = 0
	KeyTypeTimeExpiration KeyType = 1
	KeyTypeUses           KeyType = 2
	KeyTypeUnlimited      KeyType = 3
)

// EnrollmentKeyRequest is the request body for creating or updating an
// enrollment key (mirrors models.APIEnrollmentKey).
type EnrollmentKeyRequest struct {
	// Expiration is a unix timestamp (seconds); 0 means no expiration.
	Expiration        int64    `json:"expiration,omitempty"`
	UsesRemaining     int      `json:"uses_remaining,omitempty"`
	Networks          []string `json:"networks"`
	Unlimited         bool     `json:"unlimited,omitempty"`
	Tags              []string `json:"tags"`
	Type              KeyType  `json:"type"`
	Relay             string   `json:"relay,omitempty"`
	Groups            []string `json:"groups,omitempty"`
	Default           bool     `json:"default,omitempty"`
	AutoEgress        bool     `json:"auto_egress,omitempty"`
	AutoAssignGateway bool     `json:"auto_assign_gw,omitempty"`
}

// EnrollmentKey is the shape returned by CreateEnrollmentKey and
// ListEnrollmentKeys (models.EnrollmentKey on the server).
//
// Note: the server sets Tags to a single-element slice containing the key's
// internal name on this response, not the Tags originally requested.
type EnrollmentKey struct {
	Expiration        time.Time `json:"expiration"`
	UsesRemaining     int       `json:"uses_remaining"`
	Value             string    `json:"value"`
	Networks          []string  `json:"networks"`
	Unlimited         bool      `json:"unlimited"`
	Tags              []string  `json:"tags"`
	Token             string    `json:"token,omitempty"`
	Type              KeyType   `json:"type"`
	Relay             string    `json:"relay"`
	Groups            []string  `json:"groups"`
	Default           bool      `json:"default"`
	AutoEgress        bool      `json:"auto_egress"`
	AutoAssignGateway bool      `json:"auto_assign_gw"`
}

// EnrollmentKeyDetail is the persisted (schema.EnrollmentKey) shape returned
// by UpdateEnrollmentKey, RegenerateEnrollmentKeyToken, and
// GetDefaultEnrollmentKeyForNetwork.
//
// Its AutoAssignGateway field uses a different JSON key
// ("auto_assign_gateway") than EnrollmentKey's ("auto_assign_gw") — this
// mirrors a genuine inconsistency in the server's API.
type EnrollmentKeyDetail struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	Name              string    `json:"name"`
	Value             string    `json:"value"`
	Token             string    `json:"token"`
	Default           bool      `json:"default"`
	Unlimited         bool      `json:"unlimited"`
	UsesRemaining     int       `json:"uses_remaining"`
	Expiration        time.Time `json:"expiration"`
	Networks          []string  `json:"networks"`
	Tags              []string  `json:"tags"`
	GatewayID         *string   `json:"gateway_id"`
	AutoEgress        bool      `json:"auto_egress"`
	AutoAssignGateway bool      `json:"auto_assign_gateway"`
	Type              KeyType   `json:"type"`
	CreatedBy         string    `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateEnrollmentKey creates a new enrollment key.
func (c *Client) CreateEnrollmentKey(ctx context.Context, req *EnrollmentKeyRequest) (*EnrollmentKey, error) {
	var out EnrollmentKey
	if err := c.request(ctx, "POST", "/api/v1/enrollment-keys", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListEnrollmentKeys lists all enrollment keys for the tenant.
func (c *Client) ListEnrollmentKeys(ctx context.Context) ([]EnrollmentKey, error) {
	var out []EnrollmentKey
	if err := c.request(ctx, "GET", "/api/v1/enrollment-keys", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateEnrollmentKey updates an existing enrollment key by ID.
func (c *Client) UpdateEnrollmentKey(ctx context.Context, keyID string, req *EnrollmentKeyRequest) (*EnrollmentKeyDetail, error) {
	var out EnrollmentKeyDetail
	path := "/api/v1/enrollment-keys/" + url.PathEscape(keyID)
	if err := c.request(ctx, "PUT", path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RegenerateEnrollmentKeyToken regenerates the join token for an existing
// enrollment key, invalidating the previous token.
func (c *Client) RegenerateEnrollmentKeyToken(ctx context.Context, keyID string) (*EnrollmentKeyDetail, error) {
	var out EnrollmentKeyDetail
	path := "/api/v1/enrollment-keys/" + url.PathEscape(keyID) + "/regenerate-token"
	if err := c.request(ctx, "POST", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDefaultEnrollmentKeyForNetwork fetches the default enrollment key for
// a network, if one is configured.
func (c *Client) GetDefaultEnrollmentKeyForNetwork(ctx context.Context, network string) (*EnrollmentKeyDetail, error) {
	var out EnrollmentKeyDetail
	path := "/api/v1/enrollment-keys/network/" + url.PathEscape(network) + "/default"
	if err := c.request(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteEnrollmentKey deletes an enrollment key by ID.
func (c *Client) DeleteEnrollmentKey(ctx context.Context, keyID string) error {
	path := "/api/v1/enrollment-keys/" + url.PathEscape(keyID)
	return c.request(ctx, "DELETE", path, nil, nil)
}
