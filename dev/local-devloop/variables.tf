variable "ona_host" {
  type        = string
  description = "Ona host used by the provider. Defaults to the provider's standard host resolution when unset."
  default     = null
}

variable "ona_token" {
  type        = string
  description = "Ona API token used by the provider."
  sensitive   = true
  default     = null
}

variable "runner_name" {
  type        = string
  description = "Name for the runner managed by this local development module."
  default     = "terraform-provider-devloop"
}

variable "service_account_name" {
  type        = string
  description = "Name for the service account managed by this local development module."
  default     = "terraform-provider-devloop"
}

variable "service_account_valid_until" {
  type        = string
  description = "RFC3339 expiration timestamp for the managed service account."
  default     = "2099-01-01T00:00:00Z"
}

variable "group_name" {
  type        = string
  description = "Name for the group managed by this local development module."
  default     = "Terraform Provider Dev Loop"
}

variable "webhook_secret_version" {
  type        = string
  description = "User-managed webhook secret rotation marker. Change this value to rotate the secret and refresh the local test file."
  default     = "v1"
}

variable "runner_provider" {
  type        = string
  description = "Runner provider to use for the managed runner."
  default     = "aws_ec2"
}

variable "runner_region" {
  type        = string
  description = "Region hint for the managed runner."
  default     = "us-east-1"
}

variable "release_channel" {
  type        = string
  description = "Release channel for the managed runner."
  default     = "stable"
}

variable "auto_update" {
  type        = bool
  description = "Whether the managed runner should automatically update."
  default     = true
}

variable "devcontainer_image_cache_enabled" {
  type        = bool
  description = "Whether the managed runner should enable the shared devcontainer image cache."
  default     = true
}

variable "log_level" {
  type        = string
  description = "Log level for the managed runner."
  default     = "info"
}
