# Devices example

Exercises `netmaker_device` with two settings combinations
(`auto_update = true` vs. a pinned `netclient_version`, which are mutually
exclusive), plus the supporting `netmaker_network` and
`netmaker_enrollment_key` resources, and `netmaker_node` with
`is_ingress_gateway = true` to turn the auto-updating device's second-network
node into a gateway (see the extclients/ example for attaching ext clients
to it).

The example's network sets `auto_join = true`. On a network without it, a
server with device approval enabled holds a joining device as pending until
an admin approves it on the Netmaker dashboard; `netmaker_device` then can't
be created and fails with a "Device is pending approval" error naming the
network(s). Approve the device and apply again, or set `auto_join`.

Both device resources actually SSH into real machines and install
netclient — `auto_update` needs one reachable, disposable machine;
`pinned_version` needs a second, separate one (comment it out in main.tf if
you only have one available). See the top-level
[examples/README.md](../README.md) for how to point Terraform at a locally
built provider binary and set credentials.
