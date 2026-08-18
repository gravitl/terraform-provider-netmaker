// Package client is a minimal, dependency-free Go client for the parts of
// the Netmaker REST API needed to manage networks, enrollment keys, devices
// (hosts), nodes, and ext clients. Authentication (via a Netmaker user
// access token / PAT, see schema.UserAccessToken on the server) is
// optional — see WithToken — since some endpoints are callable
// unauthenticated.
//
// This client intentionally models only the fields relevant to those five
// resource types, not Netmaker's full wire schema (ACLs, egress, relays,
// gateways, RBAC, ...). Unknown response fields are ignored by
// encoding/json, so newer server versions that add fields this client
// doesn't know about will not break decoding.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client talks to a single Netmaker server on behalf of a single tenant.
type Client struct {
	baseURL    string
	token      TokenProvider
	tenantID   string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the default http.Client (e.g. for custom
// timeouts or TLS configuration).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTenantID scopes the Client to a specific tenant, sent as the
// X-Tenant-ID header on every request. It's only needed on multi-tenant
// Netmaker deployments; when omitted, the server falls back to its sole
// tenant, which is correct for single-tenant deployments.
func WithTenantID(tenantID string) Option {
	return func(c *Client) { c.tenantID = tenantID }
}

// WithToken authenticates the Client using the token supplied by the given
// TokenProvider (StaticTokenProvider for the common case of a fixed user
// access token / PAT). This is optional: without it, the Client sends no
// Authorization header, which is enough for Netmaker's public/unauthenticated
// endpoints (e.g. GetServerInfo on servers that allow it) but will fail
// against any endpoint requiring auth.
func WithToken(token TokenProvider) Option {
	return func(c *Client) { c.token = token }
}

// New creates a Client for the Netmaker server at baseURL. By default it is
// unauthenticated, suitable only for calling Netmaker's public endpoints;
// use WithToken to authenticate, and WithTenantID to scope it to a specific
// tenant on multi-tenant deployments.
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// envelope mirrors models.SuccessResponse / models.ErrorResponse, which have
// no json tags on the server and therefore serialize with capitalized keys.
type envelope struct {
	Code     int             `json:"Code"`
	Message  string          `json:"Message"`
	Response json.RawMessage `json:"Response"`
}

// request performs an HTTP request against the Netmaker API. body is
// marshaled as the JSON request body when non-nil. On success (2xx), if out
// is non-nil and the response body is non-empty, the raw response body is
// unmarshaled into out.
func (c *Client) request(ctx context.Context, method, path string, body, out any) error {
	data, err := c.rawRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("nmclient: decoding response for %s %s: %w", method, path, err)
		}
	}
	return nil
}

// requestEnveloped is like request, but expects a models.SuccessResponse
// envelope and unmarshals its Response field into out.
func (c *Client) requestEnveloped(ctx context.Context, method, path string, body, out any) error {
	data, err := c.rawRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("nmclient: decoding envelope for %s %s: %w", method, path, err)
	}
	if len(env.Response) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Response, out); err != nil {
		return fmt.Errorf("nmclient: decoding envelope response for %s %s: %w", method, path, err)
	}
	return nil
}

func (c *Client) rawRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("nmclient: encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("nmclient: building request: %w", err)
	}
	if c.token != nil {
		token, err := c.token.Token(ctx)
		if err != nil {
			return nil, fmt.Errorf("nmclient: getting token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nmclient: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nmclient: reading response for %s %s: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newAPIError(resp.StatusCode, data)
	}
	return data, nil
}
