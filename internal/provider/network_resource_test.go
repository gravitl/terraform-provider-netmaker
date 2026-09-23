package provider

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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
					"name":          config.StringVariable(netID),
					"address_range": config.StringVariable("10.50.0.0/16"),
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
					"name":          config.StringVariable(netID),
					"address_range": config.StringVariable("10.50.0.0/16"),
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
			{
				// address_range is immutable once created (see its
				// RequiresReplace plan modifier in network_resource.go) —
				// changing it must destroy and recreate the network rather
				// than update it in place, since Netmaker's own update API
				// silently ignores address changes.
				ConfigDirectory: config.TestNameDirectory(),
				ConfigVariables: config.Variables{
					"name":          config.StringVariable(netID),
					"address_range": config.StringVariable("10.51.0.0/16"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("netmaker_network.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("netmaker_network.test", "address_range", "10.51.0.0/16"),
			},
		},
	})
}
