package client

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Netmaker's wire format has "tags" as the key's name and "groups" as the
// real tag IDs; the client exposes them as Name and Tags.
func TestEnrollmentKeyRequestMarshalJSON(t *testing.T) {
	req := EnrollmentKeyRequest{
		Name:      "my-key",
		Networks:  []string{"net1", "net2"},
		Tags:      []string{"net1.web", "net2.web"},
		Type:      KeyTypeUnlimited,
		Unlimited: true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if got, want := wire["tags"], []any{"my-key"}; !reflect.DeepEqual(got, want) {
		t.Errorf("wire tags = %v, want %v", got, want)
	}
	if got, want := wire["groups"], []any{"net1.web", "net2.web"}; !reflect.DeepEqual(got, want) {
		t.Errorf("wire groups = %v, want %v", got, want)
	}
	if got, want := wire["networks"], []any{"net1", "net2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("wire networks = %v, want %v", got, want)
	}
}

func TestEnrollmentKeyRequestMarshalJSONNoName(t *testing.T) {
	// The default-key update path sets no name; the wire "tags" must stay
	// null rather than becoming [""].
	data, err := json.Marshal(EnrollmentKeyRequest{Networks: []string{"net1"}})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["tags"] != nil {
		t.Errorf("wire tags = %v, want null", wire["tags"])
	}
	if _, ok := wire["groups"]; ok {
		t.Errorf("wire groups present, want omitted")
	}
}

func TestEnrollmentKeyUnmarshalJSON(t *testing.T) {
	// A real create response from the server.
	const body = `{
		"expiration": "0001-01-01T00:00:00Z",
		"uses_remaining": 0,
		"value": "5FZZBBQV4MD7FQ3J46FWWCRAOB3J7PEO",
		"networks": ["tf-example-keys", "tf-example-keys2"],
		"unlimited": true,
		"tags": ["tf-example-multi-network"],
		"token": "abc",
		"type": 3,
		"relay": "00000000-0000-0000-0000-000000000000",
		"groups": [
			"tf-example-keys.tf-example-multi-network",
			"tf-example-keys2.tf-example-multi-network"
		],
		"default": false,
		"auto_egress": false,
		"auto_assign_gw": false
	}`

	var key EnrollmentKey
	if err := json.Unmarshal([]byte(body), &key); err != nil {
		t.Fatal(err)
	}

	if key.Name != "tf-example-multi-network" {
		t.Errorf("Name = %q, want %q", key.Name, "tf-example-multi-network")
	}
	wantTags := []string{
		"tf-example-keys.tf-example-multi-network",
		"tf-example-keys2.tf-example-multi-network",
	}
	if !reflect.DeepEqual(key.Tags, wantTags) {
		t.Errorf("Tags = %v, want %v", key.Tags, wantTags)
	}
	if key.Value != "5FZZBBQV4MD7FQ3J46FWWCRAOB3J7PEO" || key.Type != KeyTypeUnlimited || !key.Unlimited {
		t.Errorf("other fields not decoded: %+v", key)
	}
	if !reflect.DeepEqual(key.Networks, []string{"tf-example-keys", "tf-example-keys2"}) {
		t.Errorf("Networks = %v", key.Networks)
	}
}
