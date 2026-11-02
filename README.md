# Terraform Provider for Netmaker

A [Terraform](https://www.terraform.io/) provider for managing
[Netmaker](https://github.com/gravitl/netmaker) networks, enrollment keys,
tags, devices, nodes, and ext clients as code.

Requires **Netmaker v1.7.0 or newer**. The provider checks the server
version when it's configured and fails with an error on older servers
(`dev` server builds are always accepted).

## Contents

- [Features](#features)
- [Requirements](#requirements)
- [Usage](#usage)
- [Running the dev version locally](#running-the-dev-version-locally)
- [Testing](#testing)
- [Generating docs](#generating-docs)
- [Releasing](#releasing)
- [Repository layout](#repository-layout)
- [License](#license)

## Features

| Resources                                                    | Data sources                                                   |
| ------------------------------------------------------------ | -------------------------------------------------------------- |
| [`netmaker_network`](docs/resources/network.md)               | [`netmaker_network`](docs/data-sources/network.md)              |
| [`netmaker_enrollment_key`](docs/resources/enrollment_key.md) | [`netmaker_networks`](docs/data-sources/networks.md)            |
| [`netmaker_tag`](docs/resources/tag.md)                       | [`netmaker_device`](docs/data-sources/device.md)                |
| [`netmaker_device`](docs/resources/device.md)                 | [`netmaker_devices`](docs/data-sources/devices.md)              |
| [`netmaker_node`](docs/resources/node.md)                     | [`netmaker_node`](docs/data-sources/node.md)                    |
| [`netmaker_ext_client`](docs/resources/ext_client.md)         | [`netmaker_nodes`](docs/data-sources/nodes.md)                  |

`netmaker_device` provisions real machines over SSH: it detects the OS,
installs netclient, and joins the machine to a network using an
enrollment key.

Full reference documentation lives in [`docs/`](docs/index.md).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/install) 1.0+
  (the provider uses plugin protocol v6)
- A Netmaker server, v1.7.0 or newer, and a user access token (PAT)
- [Go](https://go.dev/doc/install) 1.25.8+ — only for building the
  provider from source

## Usage

```hcl
terraform {
  required_providers {
    netmaker = {
      source = "gravitl/netmaker"
    }
  }
}

provider "netmaker" {
  api_url   = "https://api.netmaker.example.com"
  api_token = var.netmaker_api_token
}

resource "netmaker_network" "office" {
  name          = "office"
  address_range = "10.101.0.0/16"
}

resource "netmaker_enrollment_key" "office" {
  networks = [netmaker_network.office.name]
  type     = "unlimited"
}
```

### Provider configuration

Every argument is optional and can be set through an environment variable
instead, which keeps credentials out of your configuration:

| Argument    | Environment variable  | Description                                                    |
| ----------- | --------------------- | -------------------------------------------------------------- |
| `api_url`   | `NETMAKER_API_URL`    | Base URL of the Netmaker server.                               |
| `api_token` | `NETMAKER_API_TOKEN`  | User access token (PAT). Omit to call only public endpoints.   |
| `tenant_id` | `NETMAKER_TENANT_ID`  | Tenant to scope requests to. Multi-tenant deployments only.    |

More complete, runnable configurations are in [`examples/`](examples/README.md).

## Running the dev version locally

To try out local changes, build the provider binary and tell Terraform to
use it instead of downloading one from the registry. Terraform's
[`dev_overrides`](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)
setting does this.

### 1. Clone the repository

```bash
git clone https://github.com/gravitl/terraform-provider-netmaker.git
```

```bash
cd terraform-provider-netmaker
```

### 2. Build the provider

Build into the `bin/` directory (gitignored).

**Linux and macOS**

```bash
go build -o bin/terraform-provider-netmaker .
```

**Windows (PowerShell)**

```powershell
go build -o bin\terraform-provider-netmaker.exe .
```

On Windows the binary must keep the `.exe` extension, or Terraform won't
find it.

### 3. Create a dev override config

Create `dev.tfrc` at the repository root (gitignored, since it contains a
machine-specific path). The path must be the absolute path to the `bin`
directory, not to the binary itself.

**Linux and macOS**

```bash
cat > dev.tfrc <<EOF
provider_installation {
  dev_overrides {
    "gravitl/netmaker" = "$(pwd)/bin"
  }
  direct {}
}
EOF
```

**Windows (PowerShell)**

```powershell
$bin = (Join-Path (Get-Location) 'bin') -replace '\\', '/'
@"
provider_installation {
  dev_overrides {
    "gravitl/netmaker" = "$bin"
  }
  direct {}
}
"@ | Set-Content -Encoding ascii dev.tfrc
```

Forward slashes are used on Windows because backslashes in HCL strings
are escape characters.

### 4. Point Terraform at the config

Set `TF_CLI_CONFIG_FILE` to the absolute path of `dev.tfrc`.

**Linux**

```bash
echo "export TF_CLI_CONFIG_FILE=\"$(pwd)/dev.tfrc\"" >> ~/.bashrc
```

```bash
source ~/.bashrc
```

**macOS** (zsh is the default shell)

```bash
echo "export TF_CLI_CONFIG_FILE=\"$(pwd)/dev.tfrc\"" >> ~/.zshrc
```

```bash
source ~/.zshrc
```

**Windows (PowerShell)**

```powershell
setx TF_CLI_CONFIG_FILE "$(Get-Location)\dev.tfrc"
```

`setx` only affects terminals opened afterwards, so open a new one before
continuing. To try the override in the current session only, run
`$env:TF_CLI_CONFIG_FILE = "$(Get-Location)\dev.tfrc"` instead.

If you'd rather not use an environment variable, put the same
`provider_installation` block in Terraform's default CLI config file:
`~/.terraformrc` on Linux and macOS, `%APPDATA%\terraform.rc` on Windows.

### 5. Run an example

Set credentials, then plan and apply one of the examples. Skip
`terraform init`: with a dev override in place Terraform uses your local
binary directly, and `init` would fail trying to look the provider up in
the registry.

**Linux and macOS**

```bash
export NETMAKER_API_URL=https://api.netmaker.example.com
export NETMAKER_API_TOKEN=<your-access-token>
```

```bash
cd examples/networks
```

```bash
terraform plan
```

**Windows (PowerShell)**

```powershell
$env:NETMAKER_API_URL = "https://api.netmaker.example.com"
$env:NETMAKER_API_TOKEN = "<your-access-token>"
```

```powershell
cd examples\networks
```

```powershell
terraform plan
```

Terraform prints a warning that provider development overrides are in
effect. That's expected.

Each example can also read its settings from a `terraform.tfvars` file;
see [`examples/README.md`](examples/README.md) for details on each one.

### 6. Iterate

After changing the code, rerun the build command from step 2. Terraform
always runs whatever binary is in `bin/`, so nothing else needs to change.

### Debugging with Delve

The provider supports Terraform's
[debugger-based workflow](https://developer.hashicorp.com/terraform/plugin/debugging#debugger-based-debugging).
Start it with the `-debug` flag, either from your IDE's debugger or with
Delve:

```bash
dlv debug . -- -debug
```

The provider prints a `TF_REATTACH_PROVIDERS` value. Set it in the
terminal where you run Terraform, and Terraform will talk to the process
under the debugger.

For verbose logs without a debugger, set `TF_LOG=DEBUG` (or
`TF_LOG_PROVIDER=DEBUG` for this provider's logs only) before running
Terraform.

## Testing

Unit tests run without any external dependencies:

```bash
go test ./...
```

Acceptance tests create real resources, so they need a real (preferably
disposable) Netmaker server. They run only when `TF_ACC` is set.

**Linux and macOS**

```bash
TF_ACC=1 NETMAKER_API_URL=https://api.netmaker.example.com NETMAKER_API_TOKEN=<token> go test ./... -v
```

**Windows (PowerShell)**

```powershell
$env:TF_ACC = "1"
$env:NETMAKER_API_URL = "https://api.netmaker.example.com"
$env:NETMAKER_API_TOKEN = "<token>"
go test ./... -v
```

Set `NETMAKER_TENANT_ID` too if the server is multi-tenant.

## Generating docs

The files in [`docs/`](docs/) are generated by
[tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs) from
the `Description` fields in each resource and data source schema. Don't
edit them by hand; after changing a schema, regenerate them:

```bash
go generate ./...
```

tfplugindocs is pinned as a Go tool in `go.mod`, so there's nothing extra
to install.

## Releasing

Releases are built by [GoReleaser](https://goreleaser.com/) through the
[release workflow](.github/workflows/release.yml). Pushing a tag that
starts with `v` builds signed binaries for Linux, macOS, Windows, and
FreeBSD and publishes them as a GitHub release:

```bash
git tag v0.1.0
```

```bash
git push origin v0.1.0
```

The workflow needs the `GPG_PRIVATE_KEY` and `PASSPHRASE` repository
secrets to sign the checksums.

## Repository layout

```
.
├── main.go                 # Provider entry point
├── internal/
│   ├── client/             # Netmaker REST API client
│   ├── provider/           # Resources, data sources, and acceptance tests
│   └── provisioner/        # SSH-based netclient/WireGuard installation for devices
├── docs/                   # Generated provider documentation
└── examples/               # Runnable example configurations
```

## License

[Apache License 2.0](LICENSE)
