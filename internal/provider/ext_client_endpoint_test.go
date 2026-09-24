package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
)

// newGatewayServer serves a gateway node whose host reports no endpoint
// until it has been asked for the host `after` times.
func newGatewayServer(t *testing.T, after int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hostCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/nodes/net1":
			json.NewEncoder(w).Encode([]map[string]string{{"id": "gw1", "hostid": "h1", "network": "net1"}})
		case "/api/hosts/h1":
			endpoint := ""
			if hostCalls.Add(1) > after {
				endpoint = "203.0.113.5"
			}
			json.NewEncoder(w).Encode(map[string]any{"Code": 200, "Response": map[string]string{"id": "h1", "endpointip": endpoint}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hostCalls
}

func fastGatewayPolling(t *testing.T, attempts int) {
	t.Helper()
	oldInterval, oldAttempts := gatewayEndpointPollInterval, gatewayEndpointAttempts
	gatewayEndpointPollInterval, gatewayEndpointAttempts = time.Millisecond, attempts
	t.Cleanup(func() { gatewayEndpointPollInterval, gatewayEndpointAttempts = oldInterval, oldAttempts })
}

func TestWaitForGatewayEndpointAppearsLater(t *testing.T) {
	fastGatewayPolling(t, 5)
	srv, calls := newGatewayServer(t, 2) // endpoint shows up on the 3rd poll
	r := &ExtClientResource{client: nmclient.New(srv.URL)}

	if err := r.waitForGatewayEndpoint(context.Background(), "net1", "gw1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("polled the host %d times, want 3", got)
	}
}

func TestWaitForGatewayEndpointNeverAppears(t *testing.T) {
	fastGatewayPolling(t, 3)
	srv, calls := newGatewayServer(t, 1000)
	r := &ExtClientResource{client: nmclient.New(srv.URL)}

	err := r.waitForGatewayEndpoint(context.Background(), "net1", "gw1")
	if err == nil {
		t.Fatal("want an error when the gateway never reports an endpoint")
	}
	for _, want := range []string{"gw1", "net1", "pending approval", "netclient"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("polled the host %d times, want 3", got)
	}
}
