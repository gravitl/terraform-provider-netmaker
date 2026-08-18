package provider

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestNetworkResource(t *testing.T) {
	netID := fmt.Sprintf("tf%d", time.Now().UnixNano()%1e9)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testPreCheck(t) },
		ProtoV6ProviderFactories: testProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.TestNameDirectory(),
				ConfigVariables: config.Variables{
					"name": config.StringVariable(netID),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netmaker_network.test", "name", netID),
					resource.TestCheckResourceAttr("netmaker_network.test", "address_range", "10.50.0.0/16"),
					resource.TestCheckResourceAttrSet("netmaker_network.test", "id"),
					resource.TestCheckResourceAttrSet("netmaker_network.test", "default_value"),
					resource.TestCheckResourceAttrSet("netmaker_network.test", "default_token"),
					resource.TestCheckResourceAttr("netmaker_network.test", "default_enrollment_key.auto_assign_gateway", "false"),
					resource.TestCheckResourceAttr("netmaker_network.test", "default_enrollment_key.tags.0", "tf-test"),
				),
			},
			{
				ConfigDirectory: config.TestNameDirectory(),
				ConfigVariables: config.Variables{
					"name": config.StringVariable(netID),
				},
				ResourceName: "netmaker_network.test",
				ImportState:  true,
				// The resource is looked up by name (netid), not by its
				// computed id (a server-assigned UUID) — without this, the
				// test harness defaults to importing by the "id" attribute
				// value, which ImportState (network_resource.go) treats as
				// a name and fails to find.
				ImportStateId:     netID,
				ImportStateVerify: true,
			},
		},
	})
}
