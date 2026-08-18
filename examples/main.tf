terraform {
  required_providers {
    netmaker = {
      source = "gravitl/netmaker"
    }
  }
}

# Values come from terraform.tfvars (see terraform.tfvars.example) if set
# there; any left blank fall back to NETMAKER_API_URL / NETMAKER_API_TOKEN /
# NETMAKER_TENANT_ID env vars.
provider "netmaker" {
  api_url   = var.netmaker_api_url
  api_token = var.netmaker_api_token
  tenant_id = var.netmaker_tenant_id
}

resource "netmaker_network" "example" {
  name          = "tf-example"
  address_range = "10.99.0.0/16"

  # Netmaker auto-creates one unlimited enrollment key per network; this
  # manages it in place instead of a separate resource + import.
  default_enrollment_key = {
    auto_assign_gateway = false
    tags                = ["tf-example"]
  }
}

# An additional, independent key for the same network (e.g. a different
# expiration/uses-limit than the default key above).
resource "netmaker_enrollment_key" "example" {
  networks = [netmaker_network.example.name]
  tags     = ["tf-example-extra"]
  type     = "unlimited"
}

# Deliberately opt-in — see deploy_device in variables.tf. This actually
# SSHes into device_host_ip and installs netclient there.
resource "netmaker_device" "example" {
  count = var.deploy_device ? 1 : 0

  name              = "tf-example-device"
  # netclient_version = var.device_netclient_version
  auto_update       = true

  deploy = {
    token = netmaker_enrollment_key.example.token
    mode = {
      ssh = {
        host_ip          = var.device_host_ip
        host_port        = var.device_host_port
        username         = var.device_username
        private_key      = var.device_private_key
        private_key_path = var.device_private_key_path
        password         = var.device_password
      }
    }
  }
}

# Joins the device to a second network, purely via the control-plane API —
# no SSH involved, since the device is already running netclient.
resource "netmaker_network" "example2" {
  count = var.deploy_device ? 1 : 0

  name          = "tf-example2"
  address_range = "10.98.0.0/16"
}

resource "netmaker_node" "example" {
  count = var.deploy_device ? 1 : 0

  device_id = netmaker_device.example[0].id
  network   = netmaker_network.example2[0].name
}

output "network_id" {
  value = netmaker_network.example.id
}

output "device_id" {
  value = var.deploy_device ? netmaker_device.example[0].id : null
}

output "default_token" {
  value     = netmaker_network.example.default_token
  sensitive = true
}

output "enrollment_key_token" {
  value     = netmaker_enrollment_key.example.token
  sensitive = true
}
