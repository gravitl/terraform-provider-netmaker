# Ext clients example

Exercises `netmaker_ext_client` with default settings and with settings
overridden (custom `dns`, `extra_allowed_ips`, `tags`, `enabled = false`),
plus the supporting `netmaker_network`, `netmaker_enrollment_key`,
`netmaker_device`, and `netmaker_node` (with `is_ingress_gateway = true`)
resources needed to have a gateway to attach ext clients to. Both the
enrollment key's and the custom ext client's `tags` reference a
`netmaker_tag` created first, in the same network the tag is scoped to
(the ext client's tag is scoped to the gateway network, not the main one)
— Netmaker doesn't auto-create tags.

The device and both ext clients actually SSH into real machines: the
device installs netclient, and each ext client installs WireGuard and
brings up its tunnel from the server-rendered config (both ext clients can
share one target machine — each gets its own WireGuard interface name).
See the top-level [examples/README.md](../README.md) for how to point
Terraform at a locally built provider binary and set credentials.
