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
brings up its tunnel from the server-rendered config. That's up to three
machines: the gateway device, and one for each ext client. The two ext
clients can't share a machine — both tunnels route the network's range, so
the second `wg-quick up` fails with `RTNETLINK answers: File exists`.
Comment out `netmaker_ext_client.custom` if you only have one client
machine.

Two things on the Netmaker side have to be in place before the ext clients
can deploy:

- The gateway device must not be pending approval. The example's network
  sets `auto_join = true` so it's added right away; on a network without it,
  an admin has to approve the device on the dashboard first.
- The gateway device's netclient must have reported a public IP. Until it
  has, the gateway has no endpoint and the ext client would get a config it
  can't connect with, so `netmaker_ext_client` waits for one (about a
  minute) and then fails with an explanation.

See the top-level [examples/README.md](../README.md) for how to point
Terraform at a locally built provider binary and set credentials.
