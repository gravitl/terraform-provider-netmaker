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

# IPv4-only network — the common case. Everything else left at server
# defaults.
resource "netmaker_network" "ipv4" {
  name          = "tf-example-ipv4"
  address_range = "10.101.0.0/16"
}

# IPv6-only network. address_range is omitted; at least one of
# address_range/address_range6 is required, not both.
resource "netmaker_network" "ipv6" {
  name           = "tf-example-ipv6"
  address_range6 = "fd00:101::/64"
}

# Dual-stack network — both an IPv4 and an IPv6 pool.
resource "netmaker_network" "dual_stack" {
  name           = "tf-example-dual"
  address_range  = "10.102.0.0/16"
  address_range6 = "fd00:102::/64"
}

# Every other network-level setting overridden from its default, plus its
# auto-created default enrollment key (see the resource's doc comment —
# Netmaker creates one unlimited key per network automatically) configured
# in place instead of left at Netmaker's defaults. jit_enabled is only
# honored at creation — Netmaker's update API silently ignores changes to
# it, so changing it here forces recreation instead of a no-op update.
resource "netmaker_network" "custom" {
  name                  = "tf-example-custom"
  address_range         = "10.103.0.0/16"
  address_range6        = "fd00:103::/64"
  auto_join             = true
  auto_remove           = true
  auto_remove_threshold = 30
  jit_enabled           = true

  # Only nodes tagged "tf-example-custom" are eligible for auto-remove
  # (use ["*"] instead to make every node in the network eligible). Like
  # default_enrollment_key.tags below, this auto-creates the tag if it
  # doesn't already exist as a netmaker_tag, since it's set in the same
  # apply that creates the network itself.
  auto_remove_tags = ["tf-example-custom"]

  # default_enrollment_key.tags auto-creates any tag that doesn't already
  # exist as a netmaker_tag (unlike netmaker_enrollment_key.tags, which
  # requires the tag to already exist — see keys/main.tf) since this key
  # is created as a side effect of the network itself, before a
  # netmaker_tag resource scoped to it could exist.
  default_enrollment_key = {
    auto_assign_gateway = false
    tags                = ["tf-example-custom"]
  }
}

output "ipv4_network_id" {
  value = netmaker_network.ipv4.id
}

output "ipv6_network_id" {
  value = netmaker_network.ipv6.id
}

output "dual_stack_network_id" {
  value = netmaker_network.dual_stack.id
}

output "custom_network_id" {
  value = netmaker_network.custom.id
}

output "custom_network_default_token" {
  value     = netmaker_network.custom.default_token
  sensitive = true
}
