# Devices example

Exercises `netmaker_device` with two settings combinations
(`auto_update = true` vs. a pinned `netclient_version`, which are mutually
exclusive), plus the supporting `netmaker_network` and
`netmaker_enrollment_key` resources, and `netmaker_node` with
`is_ingress_gateway = true` to turn the auto-updating device's second-network
node into a gateway (see the extclients/ example for attaching ext clients
to it).

Both device resources actually SSH into real machines and install
netclient — `auto_update` needs one reachable, disposable machine;
`pinned_version` needs a second, separate one (comment it out in main.tf if
you only have one available). See the top-level
[examples/README.md](../README.md) for how to point Terraform at a locally
built provider binary and set credentials.
