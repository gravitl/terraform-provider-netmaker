# Examples

Exercises this provider against a real Netmaker server, using the locally
built provider binary — no registry publish needed. Each subdirectory is a
self-contained Terraform root module (its own `terraform.tfvars`) focused
on one resource:

- [networks/](networks/) — `netmaker_network`: IPv4-only, IPv6-only,
  dual-stack, and every other network-level setting. No real machines
  involved.
- [keys/](keys/) — `netmaker_enrollment_key`: `unlimited`, `uses`,
  `time_expiration`, `auto_assign_gateway`, a key spanning multiple
  networks, and `netmaker_tag` (tags must exist before an enrollment key
  can reference them). No real machines involved.
- [devices/](devices/) — `netmaker_device` (auto-updating and
  pinned-version), plus `netmaker_node` with `is_ingress_gateway = true`
  to turn a node into a gateway. SSHes into real machine(s) and installs
  netclient.
- [extclients/](extclients/) — `netmaker_ext_client` with default and
  overridden settings, `netmaker_tag`, plus the device/node/gateway chain
  needed to attach one. SSHes into real machine(s) and installs netclient
  and WireGuard.

## 1. Build the provider binary

From the repo root:

```bash
go build -o bin/terraform-provider-netmaker .
```

## 2. Point Terraform at it (dev override)

Create `dev.tfrc` at the repo root (gitignored — machine-specific absolute
paths don't belong in version control):

```hcl
provider_installation {
  dev_overrides {
    "gravitl/netmaker" = "/absolute/path/to/repo/bin"
  }
  direct {}
}
```

Use the absolute path to the `bin` directory from step 1, not the binary
itself.

Then point Terraform at it via the `TF_CLI_CONFIG_FILE` environment
variable, set as a **permanent** user env var (`setx` on Windows, or your
shell profile on Linux/macOS) so every new terminal picks it up:

```powershell
setx TF_CLI_CONFIG_FILE "C:\path\to\repo\dev.tfrc"
```

Terraform's documented default lookup path (`~/.terraformrc` on
Linux/macOS, `%APPDATA%\terraform.rc` on Windows) works too in principle,
but in practice can end up outside whatever directories your dev
environment reliably shares with the real filesystem your shell runs in —
`dev.tfrc` living inside the repo, alongside an explicit
`TF_CLI_CONFIG_FILE`, sidesteps that ambiguity entirely. **You must open a
new terminal window** after running `setx` — it doesn't affect already-open
sessions.

With a dev override in place, Terraform runs your local binary directly
for this provider and ignores version/checksum requirements —
`terraform init` is unnecessary (and can be skipped) for it; running it
anyway will fail, since `gravitl/netmaker` isn't actually published to the
registry `init` tries to resolve it from.

## 3. Set credentials and run

Pick a subdirectory (e.g. `networks/`). Either copy its
`terraform.tfvars.example` to `terraform.tfvars` (already gitignored) and
fill in real values:

```bash
cd examples/networks
cp terraform.tfvars.example terraform.tfvars   # then edit it
terraform plan
terraform apply
terraform destroy
```

or leave `terraform.tfvars` blank and use env vars instead — both are read
by each directory's `main.tf`'s `provider "netmaker"` block, tfvars taking
precedence when non-empty:

```bash
cd examples/networks
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
always runs whatever binary currently lives at that path, for every
subdirectory.
