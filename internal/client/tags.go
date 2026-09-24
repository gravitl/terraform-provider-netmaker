package client

import (
	"context"
	"net/url"
	"time"
)

// Tag is a network-scoped tag (mirrors models.Tag). Unlike most resources
// in this provider, a Tag has no lifecycle of its own tied to any other
// resource — creating/deleting it is a first-class operation via
// /api/v1/tags, and other resources (enrollment keys, ACLs, ...) reference
// it by ID rather than owning it.
type Tag struct {
	ID        string    `json:"id"`
	TagName   string    `json:"tag_name"`
	Network   string    `json:"network"`
	ColorCode string    `json:"color_code"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// createTagRequest is the request body for creating a tag (mirrors
// models.CreateTagReq; TaggedNodes is omitted — this client doesn't
// support tagging nodes at creation time).
type createTagRequest struct {
	TagName   string `json:"tag_name"`
	Network   string `json:"network"`
	ColorCode string `json:"color_code,omitempty"`
}

// updateTagRequest is the request body for updating a tag's color (mirrors
// models.UpdateTagReq). Renaming isn't supported here since a tag's ID is
// derived from its name — that's a create/delete, not an update.
type updateTagRequest struct {
	ID        string `json:"id"`
	ColorCode string `json:"color_code"`
}

// ListTags lists all tags in a network.
func (c *Client) ListTags(ctx context.Context, network string) ([]Tag, error) {
	var out []Tag
	path := "/api/v1/tags?network=" + url.QueryEscape(network)
	if err := c.requestEnveloped(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateTag creates a new tag in a network.
func (c *Client) CreateTag(ctx context.Context, network, name, colorCode string) (*Tag, error) {
	var out Tag
	req := &createTagRequest{TagName: name, Network: network, ColorCode: colorCode}
	if err := c.requestEnveloped(ctx, "POST", "/api/v1/tags", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTag updates a tag's color code.
func (c *Client) UpdateTag(ctx context.Context, tagID, colorCode string) (*Tag, error) {
	var out Tag
	req := &updateTagRequest{ID: tagID, ColorCode: colorCode}
	if err := c.requestEnveloped(ctx, "PUT", "/api/v1/tags", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteTag deletes a tag by ID.
func (c *Client) DeleteTag(ctx context.Context, tagID string) error {
	path := "/api/v1/tags?tag_id=" + url.QueryEscape(tagID)
	return c.requestEnveloped(ctx, "DELETE", path, nil, nil)
}
