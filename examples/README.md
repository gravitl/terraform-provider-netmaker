# Local example

Exercises `netmaker_network` and `netmaker_enrollment_key` against a real
Netmaker server, using the locally built provider binary — no registry
publish needed.

## 1. Build the provider binary

From the repo root:

```bash
go build -o bin/terraform-provider-netmaker .
```

## 2. Point Terraform at it (dev override)

Create or edit `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "gravitl/netmaker" = "/home/mando/GolandProjects/netmaker-workspace/terraform-provider-netmaker/bin"
  }
  direct {}
}
```

Use the absolute path to the `bin` directory from step 1, not the binary
itself. With a dev override in place, Terraform runs your local binary
directly for this provider and ignores version/checksum requirements —
`terraform init` is unnecessary (and can be skipped) for it.

## 3. Set credentials and run

Either copy `terraform.tfvars.example` to `terraform.tfvars` (already
gitignored) and fill in real values:

```bash
cd examples
cp terraform.tfvars.example terraform.tfvars   # then edit it
terraform plan
terraform apply
terraform destroy
```

or leave `terraform.tfvars` blank and use env vars instead — both are read
by `main.tf`'s `provider "netmaker"` block, tfvars taking precedence when
non-empty:

```bash
cd examples
export NETMAKER_API_URL=https://your-netmaker-server.example.com
export NETMAKER_API_TOKEN=<a PAT>
export NETMAKER_TENANT_ID=<optional, only for multi-tenant servers>

terraform plan
terraform apply
terraform destroy
```

Terraform will print a warning that provider dev overrides are in effect —
expected, not an error.

Rebuilding after a code change just needs step 1 repeated; the dev override
always runs whatever binary currently lives at that path.
