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

# --- netmaker_device (the ext clients' gateway host) ---
# Actually SSHes into a real, disposable machine and installs netclient
# there.

variable "device_host_ip" {
  type        = string
  description = "IP address (or hostname) of the machine to provision as the gateway's netmaker_device."
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

# --- netmaker_ext_client ---
# Both ext_client resources in main.tf deploy to this same machine (can be
# the same one as device_host_ip above, or a different one) — actually
# SSHes in and installs WireGuard.

variable "ext_client_host_ip" {
  type        = string
  description = "IP address (or hostname) of the machine to deploy the ext clients' WireGuard tunnels onto."
  default     = ""
}

variable "ext_client_host_port" {
  type        = number
  description = "SSH port for the target machine. Defaults to 22."
  default     = 22
}

variable "ext_client_username" {
  type        = string
  description = "SSH username for the target machine."
  default     = ""
}

variable "ext_client_private_key" {
  type        = string
  description = "PEM-encoded SSH private key content for the target machine. Use at most one of private_key/private_key_path."
  default     = ""
  sensitive   = true
}

variable "ext_client_private_key_path" {
  type        = string
  description = "Path to a PEM-encoded SSH private key file, read from the machine running Terraform. Use at most one of private_key/private_key_path."
  default     = ""
}

variable "ext_client_password" {
  type        = string
  description = "SSH password for the target machine. Only used if neither private_key nor private_key_path is set."
  default     = ""
  sensitive   = true
}

variable "ext_client_dns" {
  type        = string
  description = "Custom DNS server for the netmaker_ext_client.custom example."
  default     = "1.1.1.1"
}

variable "ext_client_extra_allowed_ips" {
  type        = list(string)
  description = "Extra CIDRs to route through the tunnel for the netmaker_ext_client.custom example, beyond the network's own range."
  default     = ["192.168.100.0/24"]
}
