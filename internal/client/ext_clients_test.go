package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestGetExtClientConfigFileRejectsNilEndpoint(t *testing.T) {
	// What the server renders when the gateway host has no endpoint IP.
	const nilConf = "[Interface]\nAddress = 10.0.0.2/32\n\n[Peer]\nEndpoint = [<nil>]:51821\n"
	const okConf = "[Interface]\nAddress = 10.0.0.2/32\n\n[Peer]\nEndpoint = 203.0.113.5:51821\n"

	body := nilConf
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()
	c := New(srv.URL)

	if _, err := c.GetExtClientConfigFile(context.Background(), "net1", "c1"); !errors.Is(err, ErrNoGatewayEndpoint) {
		t.Errorf("nil endpoint: err = %v, want ErrNoGatewayEndpoint", err)
	}

	body = okConf
	got, err := c.GetExtClientConfigFile(context.Background(), "net1", "c1")
	if err != nil || got != okConf {
		t.Errorf("valid config: got (%q, %v), want it returned unchanged", got, err)
	}
}

func TestGetGatewayEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		v4, v6, want string
	}{
		{name: "ipv4", v4: "203.0.113.5", want: "203.0.113.5"},
		{name: "ipv6 only", v6: "2001:db8::1", want: "2001:db8::1"},
		{name: "prefers ipv4", v4: "203.0.113.5", v6: "2001:db8::1", want: "203.0.113.5"},
		{name: "none reported", want: ""},
		{name: "literal nil is not an endpoint", v4: "<nil>", v6: "<nil>", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/nodes/net1":
					json.NewEncoder(w).Encode([]map[string]string{{"id": "gw1", "hostid": "h1", "network": "net1"}})
				case "/api/hosts/h1":
					json.NewEncoder(w).Encode(map[string]any{"Code": 200, "Response": map[string]string{
						"id": "h1", "endpointip": tt.v4, "endpointipv6": tt.v6,
					}})
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			got, err := New(srv.URL).GetGatewayEndpoint(context.Background(), "net1", "gw1")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// fakeExtClientServer mimics the real server: POST (create) ignores the
// requested enabled/tags and always returns an enabled client with no tags,
// while PUT (update) applies them.
func fakeExtClientServer(t *testing.T, calls *[]string, putBodies *[]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, r.Method)
		switch r.Method {
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{"clientid": "c1", "network": "net1", "enabled": true, "tags": map[string]any{}})
		case http.MethodPut:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			*putBodies = append(*putBodies, body)
			enabled, _ := body["enabled"].(bool)
			tags, _ := body["tags"].(map[string]any)
			if tags == nil {
				tags = map[string]any{}
			}
			json.NewEncoder(w).Encode(map[string]any{"clientid": "c1", "network": "net1", "enabled": enabled, "tags": tags})
		}
	}))
}

func TestCreateExtClientAppliesIgnoredFieldsViaUpdate(t *testing.T) {
	tests := []struct {
		name        string
		req         ExtClientRequest
		wantCalls   []string
		wantEnabled bool
		wantTags    int
	}{
		{name: "enabled without tags needs no update", req: ExtClientRequest{Enabled: true}, wantCalls: []string{"POST"}, wantEnabled: true},
		{name: "disabled is applied by update", req: ExtClientRequest{Enabled: false}, wantCalls: []string{"POST", "PUT"}, wantEnabled: false},
		{name: "tags are applied by update", req: ExtClientRequest{Enabled: true, Tags: map[string]struct{}{"net1.web": {}}}, wantCalls: []string{"POST", "PUT"}, wantEnabled: true, wantTags: 1},
		{name: "disabled with tags", req: ExtClientRequest{Enabled: false, Tags: map[string]struct{}{"net1.web": {}}}, wantCalls: []string{"POST", "PUT"}, wantEnabled: false, wantTags: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			var putBodies []map[string]any
			srv := fakeExtClientServer(t, &calls, &putBodies)
			defer srv.Close()

			got, err := New(srv.URL).CreateExtClient(context.Background(), "net1", "gw1", &tt.req)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", calls, tt.wantCalls)
			}
			if got.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tt.wantEnabled)
			}
			if len(got.Tags) != tt.wantTags {
				t.Errorf("Tags = %v, want %d", got.Tags, tt.wantTags)
			}
			// The update endpoint reads a missing "enabled" as false, so it must
			// always be present in the body, even when false.
			for _, b := range putBodies {
				if _, ok := b["enabled"]; !ok {
					t.Errorf("update body has no \"enabled\" field: %v", b)
				}
			}
		})
	}
}
