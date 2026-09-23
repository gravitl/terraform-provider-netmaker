package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DeployModel maps the netmaker_device resource's `deploy` nested
// attribute: the enrollment-key token to join with, plus the mechanism
// (mode) used to reach the machine and perform the install/join.
type DeployModel struct {
	Token types.String `tfsdk:"token"`
	Mode  *ModeModel   `tfsdk:"mode"`
}

func deploySchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: "How to provision this device: the enrollment-key token to join with, and the mechanism used to reach the machine.",
		Required:    true,
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Description: "Join token from a netmaker_enrollment_key (its `token` attribute). Determines which network(s) the device joins.",
				Required:    true,
				Sensitive:   true,
			},
			"mode": modeSchema(),
		},
	}
}
