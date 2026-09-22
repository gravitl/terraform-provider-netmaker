# Enrollment keys example

Exercises `netmaker_enrollment_key` with each key type (`unlimited`,
`uses`, `time_expiration`), `auto_assign_gateway`, and a key spanning
multiple networks. Also exercises `netmaker_tag`: each key's `tags` entry
references a `netmaker_tag` created first — Netmaker doesn't auto-create
tags, and an enrollment key referencing one that doesn't exist would
otherwise fail (a tag spanning multiple networks needs one `netmaker_tag`
per network, since a tag is scoped to a single network).

No real machines involved — safe to `terraform apply` against any test
server. Before applying, set `key_expiration_unix` (see
terraform.tfvars.example) to a real future timestamp. See the top-level
[examples/README.md](../README.md) for how to point Terraform at a locally
built provider binary and set credentials.
