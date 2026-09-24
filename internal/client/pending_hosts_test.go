package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestPendingNetworksForHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/networks":
			json.NewEncoder(w).Encode([]map[string]string{{"netid": "net1"}, {"netid": "net2"}, {"netid": "net3"}})
		case "/api/v1/pending_hosts":
			pending := map[string][]map[string]string{
				"net1": {{"id": "p1", "host_name": "my-device", "network": "net1"}},
				"net2": {{"id": "p2", "host_name": "someone-else", "network": "net2"}},
				"net3": {{"id": "p3", "host_name": "my-device", "network": "net3"}},
			}
			json.NewEncoder(w).Encode(map[string]any{"Code": 200, "Response": pending[r.URL.Query().Get("network")]})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(srv.URL)

	got, err := c.PendingNetworksForHost(context.Background(), "my-device")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"net1", "net3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("pending in %v, want %v", got, want)
	}

	got, err = c.PendingNetworksForHost(context.Background(), "not-pending")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("pending in %v, want none", got)
	}
}
