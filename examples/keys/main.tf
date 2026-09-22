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
  name          = "tf-example-keys"
  address_range = "10.104.0.0/16"
}

resource "netmaker_network" "example2" {
  name          = "tf-example-keys2"
  address_range = "10.105.0.0/16"
}

# netmaker_enrollment_key.tags requires each tag to already exist as a
# netmaker_tag in every network the key covers — Netmaker doesn't
# auto-create tags, and unlike netmaker_network's default_enrollment_key
# (see networks/main.tf), there's no reason this key can't just depend on
# the tag being created first. Referencing .name (rather than a literal
# string) is what creates that dependency.
resource "netmaker_tag" "unlimited" {
  network = netmaker_network.example.name
  name    = "tf-example-unlimited"
}

resource "netmaker_tag" "uses" {
  network = netmaker_network.example.name
  name    = "tf-example-uses"
}

resource "netmaker_tag" "time_expiration" {
  network = netmaker_network.example.name
  name    = "tf-example-time-expiration"
}

resource "netmaker_tag" "auto_gateway" {
  network = netmaker_network.example.name
  name    = "tf-example-auto-gateway"
}

# A tag is scoped to one network, so a key covering multiple networks
# (multi_network below) needs one netmaker_tag per network, even when
# using the same name in each.
resource "netmaker_tag" "multi_network_1" {
  network = netmaker_network.example.name
  name    = "tf-example-multi-network"
}

resource "netmaker_tag" "multi_network_2" {
  network = netmaker_network.example2.name
  name    = "tf-example-multi-network"
}

# Unlimited uses, no expiration.
resource "netmaker_enrollment_key" "unlimited" {
  networks = [netmaker_network.example.name]
  type     = "unlimited"
  tags     = [netmaker_tag.unlimited.name]
}

# A fixed number of uses; the server decrements uses_remaining on each
# device that joins with it.
resource "netmaker_enrollment_key" "uses" {
  networks       = [netmaker_network.example.name]
  type           = "uses"
  uses_remaining = 5
  tags           = [netmaker_tag.uses.name]
}

# Expires at a fixed point in time instead of a use count. Update
# key_expiration_unix (see variables.tf) to a real future timestamp before
# applying — the default is just a placeholder.
resource "netmaker_enrollment_key" "time_expiration" {
  networks        = [netmaker_network.example.name]
  type            = "time_expiration"
  expiration_unix = var.key_expiration_unix
  tags            = [netmaker_tag.time_expiration.name]
}

# Devices enrolled with this key auto-select a gateway instead of needing
# one pinned via gateway_id — see the devices/ and extclients/ examples for
# how to turn a node into a gateway (netmaker_node's is_ingress_gateway).
resource "netmaker_enrollment_key" "auto_gateway" {
  networks            = [netmaker_network.example.name]
  type                = "unlimited"
  auto_assign_gateway = true
  tags                = [netmaker_tag.auto_gateway.name]
}

# Covers more than one network at once — a device joining with this key
# gets a Node in every listed network.
resource "netmaker_enrollment_key" "multi_network" {
  networks   = [netmaker_network.example.name, netmaker_network.example2.name]
  type       = "unlimited"
  tags       = [netmaker_tag.multi_network_1.name]
  depends_on = [netmaker_tag.multi_network_2]
}

output "unlimited_key_token" {
  value     = netmaker_enrollment_key.unlimited.token
  sensitive = true
}

output "uses_key_token" {
  value     = netmaker_enrollment_key.uses.token
  sensitive = true
}

output "time_expiration_key_token" {
  value     = netmaker_enrollment_key.time_expiration.token
  sensitive = true
}

output "auto_gateway_key_token" {
  value     = netmaker_enrollment_key.auto_gateway.token
  sensitive = true
}

output "multi_network_key_token" {
  value     = netmaker_enrollment_key.multi_network.token
  sensitive = true
}
