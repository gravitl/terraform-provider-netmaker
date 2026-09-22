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
  name          = "tf-example-devices"
  address_range = "10.106.0.0/16"
}

# netmaker_enrollment_key.tags requires the tag to already exist as a
# netmaker_tag — Netmaker doesn't auto-create tags. Referencing .name
# (rather than a literal string) is what creates that dependency.
resource "netmaker_tag" "example" {
  network = netmaker_network.example.name
  name    = "tf-example-devices"
}

resource "netmaker_enrollment_key" "example" {
  networks = [netmaker_network.example.name]
  type     = "unlimited"
  tags     = [netmaker_tag.example.name]
}

# Auto-updating device — netclient keeps itself current, no pinned
# version. Actually SSHes into device_host_ip and installs netclient
# there, joining it to tf-example-devices via the enrollment key above.
resource "netmaker_device" "auto_update" {
  name        = "tf-example-device-auto"
  auto_update = true

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

# Pinned-version device — installs a specific netclient release instead of
# auto-updating (netclient_version and auto_update are mutually exclusive).
# Requires a second, separate machine — comment this out if you only have
# one test machine available.
resource "netmaker_device" "pinned_version" {
  name              = "tf-example-device-pinned"
  netclient_version = var.device_pinned_netclient_version

  deploy = {
    token = netmaker_enrollment_key.example.token
    mode = {
      ssh = {
        host_ip          = var.device_pinned_host_ip
        host_port        = var.device_pinned_host_port
        username         = var.device_pinned_username
        private_key      = var.device_pinned_private_key
        private_key_path = var.device_pinned_private_key_path
        password         = var.device_pinned_password
      }
    }
  }
}

# A second network for the gateway node, joined purely via the
# control-plane API (netmaker_node), not the enrollment key above — the
# auto_update device already joined tf-example-devices during install, and
# that API only attaches an already-registered device to an *additional*
# network.
resource "netmaker_network" "gateway" {
  name          = "tf-example-devices-gw"
  address_range = "10.107.0.0/16"
}

# Joins the auto_update device to the second network and turns that node
# into an ingress (remote-access) gateway — see the extclients/ example for
# attaching ext clients to it.
resource "netmaker_node" "gateway" {
  device_id          = netmaker_device.auto_update.id
  network            = netmaker_network.gateway.name
  is_ingress_gateway = true
}

output "network_id" {
  value = netmaker_network.example.id
}

output "auto_update_device_id" {
  value = netmaker_device.auto_update.id
}

output "pinned_version_device_id" {
  value = netmaker_device.pinned_version.id
}

output "gateway_node_id" {
  value = netmaker_node.gateway.id
}

output "default_token" {
  value     = netmaker_network.example.default_token
  sensitive = true
}

output "enrollment_key_token" {
  value     = netmaker_enrollment_key.example.token
  sensitive = true
}
