package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	nmclient "github.com/gravitl/terraform-provider-netmaker/internal/client"
)

// newTagServer serves GET /api/v1/tags?network=<n> from the given
// network -> tag names map, in Netmaker's response envelope.
func newTagServer(t *testing.T, tags map[string][]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		network := r.URL.Query().Get("network")
		list := []map[string]string{}
		for _, name := range tags[network] {
			list = append(list, map[string]string{"id": network + "." + name, "tag_name": name, "network": network})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Code": 200, "Response": list})
	}))
}

func TestEnrollmentKeyValidateTagIDs(t *testing.T) {
	srv := newTagServer(t, map[string][]string{
		"net1": {"web", "db"},
		"net2": {"web"},
	})
	defer srv.Close()
	r := &EnrollmentKeyResource{client: nmclient.New(srv.URL)}

	tests := []struct {
		name     string
		networks []string
		tags     []string
		wantErr  string
	}{
		{name: "no tags", networks: []string{"net1"}},
		{name: "id, single network", networks: []string{"net1"}, tags: []string{"net1.db"}},
		{name: "one id on a multi-network key", networks: []string{"net1", "net2"}, tags: []string{"net1.web"}},
		{name: "ids for both networks", networks: []string{"net1", "net2"}, tags: []string{"net1.web", "net2.web"}},
		{name: "plain name is not an id", networks: []string{"net1"}, tags: []string{"web"}, wantErr: "not found among"},
		{name: "id from a network the key doesn't cover", networks: []string{"net1"}, tags: []string{"net2.web"}, wantErr: "not found among"},
		{name: "missing tag", networks: []string{"net1", "net2"}, tags: []string{"net2.db"}, wantErr: "not found among"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.validateTagIDs(context.Background(), tt.networks, tt.tags)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTagSetToStringList(t *testing.T) {
	ctx := context.Background()
	set := map[string]struct{}{"net.b": {}, "net.a": {}, "net.c": {}}

	toStrings := func(l types.List) []string {
		var out []string
		l.ElementsAs(ctx, &out, false)
		return out
	}
	prior := func(ids ...string) types.List {
		l, _ := types.ListValueFrom(ctx, types.StringType, ids)
		return l
	}

	// Same tags as the prior list: keep the configured order.
	got, _ := tagSetToStringList(ctx, set, prior("net.c", "net.a", "net.b"))
	if want := []string{"net.c", "net.a", "net.b"}; !reflect.DeepEqual(toStrings(got), want) {
		t.Errorf("same set: got %v, want %v", toStrings(got), want)
	}

	// Different tags than the prior list: fall back to sorted server tags.
	got, _ = tagSetToStringList(ctx, set, prior("net.a"))
	if want := []string{"net.a", "net.b", "net.c"}; !reflect.DeepEqual(toStrings(got), want) {
		t.Errorf("different set: got %v, want %v", toStrings(got), want)
	}

	// No prior list (e.g. import): sorted.
	got, _ = tagSetToStringList(ctx, set, types.ListNull(types.StringType))
	if want := []string{"net.a", "net.b", "net.c"}; !reflect.DeepEqual(toStrings(got), want) {
		t.Errorf("null prior: got %v, want %v", toStrings(got), want)
	}
}
