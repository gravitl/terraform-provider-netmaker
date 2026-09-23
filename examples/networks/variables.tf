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
