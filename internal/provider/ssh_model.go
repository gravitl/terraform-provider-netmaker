package provider

import (
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/gravitl/terraform-provider-netmaker/internal/provisioner"
)

// SSHModel maps the `ssh` nested attribute inside `mode` — one deploy
// mechanism among potentially several (see ModeModel).
type SSHModel struct {
	HostIP         types.String `tfsdk:"host_ip"`
	HostPort       types.Int64  `tfsdk:"host_port"`
	Username       types.String `tfsdk:"username"`
	PrivateKey     types.String `tfsdk:"private_key"`
	PrivateKeyPath types.String `tfsdk:"private_key_path"`
	Password       types.String `tfsdk:"password"`
}

func sshSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: "Connect and deploy over SSH.",
		Required:    true,
		Attributes: map[string]schema.Attribute{
			"host_ip": schema.StringAttribute{
				Description: "IP address (or hostname) of the target machine.",
				Required:    true,
			},
			"host_port": schema.Int64Attribute{
				Description: "SSH port. Defaults to 22.",
				Optional:    true,
				Computed:    true,
			},
			"username": schema.StringAttribute{
				Description: "SSH username.",
				Required:    true,
			},
			"private_key": schema.StringAttribute{
				Description: "PEM-encoded SSH private key content. Mutually exclusive with private_key_path; use exactly one of private_key/private_key_path, or password.",
				Optional:    true,
				Sensitive:   true,
			},
			"private_key_path": schema.StringAttribute{
				Description: "Path to a PEM-encoded SSH private key file, read from the machine running Terraform. Mutually exclusive with private_key; use exactly one of private_key/private_key_path, or password.",
				Optional:    true,
			},
			"password": schema.StringAttribute{
				Description: "SSH password. Only used if neither private_key nor private_key_path is set.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (m SSHModel) validate() error {
	privateKey := m.PrivateKey.ValueString()
	privateKeyPath := m.PrivateKeyPath.ValueString()
	password := m.Password.ValueString()

	if privateKey != "" && privateKeyPath != "" {
		return fmt.Errorf("ssh: specify only one of private_key or private_key_path")
	}
	hasKey := privateKey != "" || privateKeyPath != ""
	if hasKey && password != "" {
		return fmt.Errorf("ssh: password cannot be set when private_key or private_key_path is set")
	}
	if !hasKey && password == "" {
		return fmt.Errorf("ssh: one of private_key, private_key_path, or password must be set")
	}
	return nil
}

func (m SSHModel) toProvisionerConfig() (provisioner.Config, error) {
	if err := m.validate(); err != nil {
		return provisioner.Config{}, err
	}

	privateKey := m.PrivateKey.ValueString()
	if path := m.PrivateKeyPath.ValueString(); path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return provisioner.Config{}, fmt.Errorf("ssh: reading private_key_path %q: %w", path, err)
		}
		privateKey = string(content)
	}

	port := m.HostPort.ValueInt64()
	if port == 0 {
		port = 22
	}
	return provisioner.Config{
		Host:       m.HostIP.ValueString(),
		Port:       port,
		User:       m.Username.ValueString(),
		PrivateKey: privateKey,
		Password:   m.Password.ValueString(),
	}, nil
}

// ModeModel maps the shared `mode` nested attribute used by
// netmaker_device and netmaker_ext_client — the extensible container for
// deploy mechanisms. SSH is the only one today; a future mechanism (e.g.
// AWS SSM) would be added here as a sibling attribute, not by changing
// this shape.
type ModeModel struct {
	SSH *SSHModel `tfsdk:"ssh"`
}

func modeSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: "Deploy mechanism. SSH is currently the only one supported.",
		Required:    true,
		Attributes: map[string]schema.Attribute{
			"ssh": sshSchema(),
		},
	}
}

func (m ModeModel) validate() error {
	if m.SSH == nil {
		return fmt.Errorf("mode: no deploy mechanism configured (ssh is currently required)")
	}
	return m.SSH.validate()
}

func (m ModeModel) toProvisionerConfig() (provisioner.Config, error) {
	if m.SSH == nil {
		return provisioner.Config{}, fmt.Errorf("mode: no deploy mechanism configured (ssh is currently required)")
	}
	return m.SSH.toProvisionerConfig()
}
