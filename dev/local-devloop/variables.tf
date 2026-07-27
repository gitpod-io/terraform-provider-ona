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

variable "enable_ai_budgets" {
  type        = bool
  description = "Whether to exercise enterprise AI budget resources. Enabling this changes organization billing policy and requires suitable Billing permissions."
  default     = false
}

variable "ai_budget_team_id" {
  type        = string
  description = "Existing team UUID used for mode-specific AI budget resources. Leave null to skip team budgets."
  default     = null
  nullable    = true
}

variable "organization_monthly_credit_limit" {
  type        = number
  description = "Whole-credit organization default used by the AI budget dev loop."
  default     = 5000
}

variable "organization_monthly_cost_limit_microunits" {
  type        = number
  description = "Organization BYOK default in currency microunits used by the AI budget dev loop."
  default     = 100000000
}

variable "service_account_monthly_credit_limit" {
  type        = number
  description = "Whole-credit direct override for the devloop service account."
  default     = 750
}

variable "team_credit_budget" {
  type        = number
  description = "Whole-credit soft budget applied to the configured devloop team."
  default     = 1500
}

variable "team_cost_budget_microunits" {
  type        = number
  description = "BYOK soft budget in currency microunits applied to the configured devloop team."
  default     = 50000000
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
