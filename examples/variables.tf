variable "netmaker_api_url" {
  type        = string
  description = "Netmaker server URL. Leave blank to fall back to the NETMAKER_API_URL env var."
  default     = ""
}

variable "netmaker_api_token" {
  type        = string
  description = "Netmaker user access token (PAT). Leave blank to fall back to the NETMAKER_API_TOKEN env var."
  default     = ""
  sensitive   = true
}

variable "netmaker_tenant_id" {
  type        = string
  description = "Netmaker tenant ID, only needed on multi-tenant servers. Leave blank to fall back to the NETMAKER_TENANT_ID env var."
  default     = ""
}

# --- netmaker_device (optional; deliberately opt-in) ---
# Unlike the other resources above, netmaker_device actually SSHes into a
# real machine and installs software on it. It's disabled by default so a
# bare `terraform apply` never does that unintentionally — set
# deploy_device = true (and fill in the connection details) to try it.

variable "deploy_device" {
  type        = bool
  description = "Set true to include the netmaker_device example (requires a real, reachable machine)."
  default     = false
}

variable "device_host_ip" {
  type        = string
  description = "IP address (or hostname) of the machine to provision as a netmaker_device."
  default     = ""
}

variable "device_host_port" {
  type        = number
  description = "SSH port for the target machine. Defaults to 22."
  default     = 22
}

variable "device_username" {
  type        = string
  description = "SSH username for the target machine."
  default     = ""
}

variable "device_private_key" {
  type        = string
  description = "PEM-encoded SSH private key content for the target machine. Use at most one of private_key/private_key_path."
  default     = ""
  sensitive   = true
}

variable "device_private_key_path" {
  type        = string
  description = "Path to a PEM-encoded SSH private key file, read from the machine running Terraform. Use at most one of private_key/private_key_path."
  default     = ""
}

variable "device_password" {
  type        = string
  description = "SSH password for the target machine. Only used if neither private_key nor private_key_path is set."
  default     = ""
  sensitive   = true
}

variable "device_netclient_version" {
  type        = string
  description = "netclient release version to install, e.g. \"v1.2.0\" (see https://github.com/gravitl/netclient/releases)."
  default     = ""
}
