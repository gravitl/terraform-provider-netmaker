package provider

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestEnrollmentKeyResource(t *testing.T) {
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
					resource.TestCheckResourceAttr("netmaker_enrollment_key.test", "name", netID+"-key"),
					resource.TestCheckResourceAttr("netmaker_enrollment_key.test", "tags.0", netID+".tf-test"),
					resource.TestCheckResourceAttr("netmaker_tag.test", "id", netID+".tf-test"),
					resource.TestCheckResourceAttr("netmaker_enrollment_key.test", "type", "unlimited"),
					resource.TestCheckResourceAttr("netmaker_enrollment_key.test", "networks.0", netID),
					resource.TestCheckResourceAttrSet("netmaker_enrollment_key.test", "value"),
					resource.TestCheckResourceAttrSet("netmaker_enrollment_key.test", "token"),
				),
			},
		},
	})
}
