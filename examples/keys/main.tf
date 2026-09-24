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

# netmaker_enrollment_key.tags takes tag ids, and each tag must already
# exist as a netmaker_tag in one of the key's networks — Netmaker doesn't
# auto-create tags, and unlike netmaker_network's default_enrollment_key
# (see networks/main.tf), there's no reason this key can't just depend on
# the tag being created first. Referencing the tag's .id is what makes
# Terraform create it first.
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

# A tag belongs to one network — see multi_network below for how a key
# covering several networks picks which network's tag it means.
resource "netmaker_tag" "multi_network" {
  network = netmaker_network.example.name
  name    = "tf-example-multi-network"
}

# Unlimited uses, no expiration.
resource "netmaker_enrollment_key" "unlimited" {
  name     = "tf-example-unlimited-key"
  networks = [netmaker_network.example.name]
  type     = "unlimited"
  tags     = [netmaker_tag.unlimited.id]
}

# A fixed number of uses; the server decrements uses_remaining on each
# device that joins with it.
resource "netmaker_enrollment_key" "uses" {
  name           = "tf-example-uses-key"
  networks       = [netmaker_network.example.name]
  type           = "uses"
  uses_remaining = 5
  tags           = [netmaker_tag.uses.id]
}

# Expires at a fixed point in time instead of a use count. Update
# key_expiration_unix (see variables.tf) to a real future timestamp before
# applying — the default is just a placeholder.
resource "netmaker_enrollment_key" "time_expiration" {
  name            = "tf-example-expiring-key"
  networks        = [netmaker_network.example.name]
  type            = "time_expiration"
  expiration_unix = var.key_expiration_unix
  tags            = [netmaker_tag.time_expiration.id]
}

# Devices enrolled with this key auto-select a gateway instead of needing
# one pinned via gateway_id — see the devices/ and extclients/ examples for
# how to turn a node into a gateway (netmaker_node's is_ingress_gateway).
resource "netmaker_enrollment_key" "auto_gateway" {
  name                = "tf-example-auto-gateway-key"
  networks            = [netmaker_network.example.name]
  type                = "unlimited"
  auto_assign_gateway = true

  # Shares the tag with the "unlimited" key above — a key's name and its
  # tags are independent, so any number of keys can use the same tag.
  tags = [netmaker_tag.unlimited.id]
}

# Covers more than one network at once — a device joining with this key
# gets a Node in every listed network. On a multi-network key, tags are
# referenced by id ("<network>.<name>") rather than plain name, since a
# name alone doesn't say which network's tag is meant. Only the tag listed
# here is applied: nodes in tf-example-keys get it, nodes in
# tf-example-keys2 get none.
resource "netmaker_enrollment_key" "multi_network" {
  name     = "tf-example-multi-network-key"
  networks = [netmaker_network.example.name, netmaker_network.example2.name]
  type     = "unlimited"
  tags     = [netmaker_tag.multi_network.id]
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
