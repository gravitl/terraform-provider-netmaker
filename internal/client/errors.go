package client

import (
	"encoding/json"
	"fmt"
)

// APIError is returned for any non-2xx response from the Netmaker API.
type APIError struct {
	StatusCode int
	Code       int
	Message    string
	Response   json.RawMessage
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("nmclient: http %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("nmclient: http %d", e.StatusCode)
}

func newAPIError(status int, body []byte) error {
	var env envelope
	if err := json.Unmarshal(body, &env); err == nil && (env.Message != "" || len(env.Response) > 0) {
		return &APIError{StatusCode: status, Code: env.Code, Message: env.Message, Response: env.Response}
	}
	return &APIError{StatusCode: status, Message: string(body)}
}
