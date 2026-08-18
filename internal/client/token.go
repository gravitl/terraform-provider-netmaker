package client

import "context"

// TokenProvider supplies the bearer token used to authenticate requests. It
// is consulted on every request (not just once at Client construction), so
// implementations may fetch or refresh a token dynamically.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// StaticTokenProvider is a TokenProvider that always returns the same
// token — the common case of authenticating with a fixed user access
// token (PAT).
type StaticTokenProvider string

func (t StaticTokenProvider) Token(_ context.Context) (string, error) {
	return string(t), nil
}
