# Networks example

Exercises `netmaker_network` with various address-family and settings
combinations: IPv4-only, IPv6-only, dual-stack, and every other
network-level setting (auto_join, auto_remove, auto_remove_threshold,
auto_remove_tags, jit_enabled) plus its auto-created default enrollment
key. auto_remove_tags demonstrates the same auto-create-if-missing tag
behavior as default_enrollment_key.tags — both are attributes of the
network resource itself, so the tag can't have been created first via a
separate netmaker_tag resource.

No real machines involved — safe to `terraform apply` against any test
server. See the top-level [examples/README.md](../README.md) for how to
point Terraform at a locally built provider binary and set credentials.
