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

# auto_join = true adds devices that join with this network's enrollment key
# immediately. Without it, a server with device approval enabled holds each
# device as pending until an admin approves it on the Netmaker dashboard —
# and netmaker_device can't be created until then.
resource "netmaker_network" "example" {
  name          = "tf-example-extclients"
  address_range = "10.108.0.0/16"
  auto_join     = true
}

# netmaker_enrollment_key.tags takes tag ids, and the tag must already exist
# as a netmaker_tag — Netmaker doesn't auto-create tags. Referencing the
# tag's .id is also what makes Terraform create it first.
resource "netmaker_tag" "example" {
  network = netmaker_network.example.name
  name    = "tf-example-extclients"
}

resource "netmaker_enrollment_key" "example" {
  name     = "tf-example-extclients-key"
  networks = [netmaker_network.example.name]
  type     = "unlimited"
  tags     = [netmaker_tag.example.id]
}

# Actually SSHes into device_host_ip and installs netclient there, joining
# it to tf-example-extclients via the enrollment key above.
resource "netmaker_device" "example" {
  name        = "tf-example-extclients-device"
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

# A second network for the gateway node, joined purely via the
# control-plane API (netmaker_node), not the enrollment key above — the
# device already joined tf-example-extclients during install, and that API
# only attaches an already-registered device to an *additional* network.
resource "netmaker_network" "gateway" {
  name          = "tf-example-extclients-gw"
  address_range = "10.109.0.0/16"
}

# Joins the device to the second network and turns that node into an
# ingress (remote-access) gateway — the attach point ext clients connect
# through.
resource "netmaker_node" "gateway" {
  device_id          = netmaker_device.example.id
  network            = netmaker_network.gateway.name
  is_ingress_gateway = true
}

# Minimal ext client — everything left at server defaults. Actually SSHes
# into ext_client_host_ip, installs WireGuard, and brings up the tunnel
# from the server-rendered config.
resource "netmaker_ext_client" "default" {
  network         = netmaker_node.gateway.network
  gateway_node_id = netmaker_node.gateway.id

  mode = {
    ssh = {
      host_ip          = var.ext_client_host_ip
      host_port        = var.ext_client_host_port
      username         = var.ext_client_username
      private_key      = var.ext_client_private_key
      private_key_path = var.ext_client_private_key_path
      password         = var.ext_client_password
    }
  }
}

# netmaker_ext_client.tags also requires the tag to already exist —
# scoped to the gateway network (tf-example-extclients-gw), which is what
# ext_client.custom.network resolves to, not the main network above.
resource "netmaker_tag" "extclient_custom" {
  network = netmaker_network.gateway.name
  name    = "tf-example-extclient-custom"
}

# Ext client with settings overridden: custom DNS, extra routed IPs, tags,
# and disabled (Netmaker keeps the config but excludes it from active peer
# updates until enabled = true). Needs its own target machine, separate from
# netmaker_ext_client.default's: both tunnels route the whole network range,
# and one machine can't have two interfaces claiming the same route (the
# second `wg-quick up` fails with "RTNETLINK answers: File exists").
# Comment this resource out if you only have one client machine.
resource "netmaker_ext_client" "custom" {
  network         = netmaker_node.gateway.network
  gateway_node_id = netmaker_node.gateway.id

  dns               = var.ext_client_dns
  extra_allowed_ips = var.ext_client_extra_allowed_ips
  tags              = [netmaker_tag.extclient_custom.id]
  enabled           = false

  mode = {
    ssh = {
      host_ip          = var.ext_client_custom_host_ip
      host_port        = var.ext_client_custom_host_port
      username         = var.ext_client_custom_username
      private_key      = var.ext_client_custom_private_key
      private_key_path = var.ext_client_custom_private_key_path
      password         = var.ext_client_custom_password
    }
  }
}

output "network_id" {
  value = netmaker_network.example.id
}

output "device_id" {
  value = netmaker_device.example.id
}

output "gateway_node_id" {
  value = netmaker_node.gateway.id
}

output "default_ext_client_id" {
  value = netmaker_ext_client.default.id
}

output "custom_ext_client_id" {
  value = netmaker_ext_client.custom.id
}
