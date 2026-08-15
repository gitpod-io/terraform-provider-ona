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

variable "runner_token_version" {
  type        = string
  description = "User-managed runner token rotation marker. Change this value to replace the resource and mint a new token."
  default     = "v1"
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

variable "enable_git_authentication" {
  type        = bool
  description = "Whether to associate the dev-loop service account with the GitHub PAT SCM integration."
  default     = false
}

variable "git_personal_access_token" {
  type        = string
  description = "GitHub personal access token sent to Ona when Git authentication is enabled."
  sensitive   = true
  ephemeral   = true
  default     = null
}

variable "git_personal_access_token_version" {
  type        = string
  description = "User-managed rotation marker for the GitHub personal access token."
  default     = "v1"
}

variable "group_name" {
  type        = string
  description = "Name for the group managed by this local development module."
  default     = "Terraform Provider Dev Loop"
}

variable "runner_sharing_role" {
  type        = string
  description = "Runner role assigned to an empty devloop group. Set to user or admin to enable runner sharing lifecycle tests."
  default     = null
  nullable    = true

  validation {
    condition     = var.runner_sharing_role == null || contains(["user", "admin"], var.runner_sharing_role)
    error_message = "runner_sharing_role must be user, admin, or null."
  }
}

variable "automation_sharing_automation_id" {
  type        = string
  description = "Existing Automation ID used to exercise custom-group sharing. Requires an Enterprise organization when set."
  default     = null
  nullable    = true
}

variable "project_sharing_role" {
  type        = string
  description = "Project role assigned to the devloop group. Change this value to exercise replacement."
  default     = "editor"

  validation {
    condition     = contains(["user", "editor", "admin"], var.project_sharing_role)
    error_message = "project_sharing_role must be user, editor, or admin."
  }
}

variable "enable_ai_budgets" {
  type        = bool
  description = "Whether to exercise enterprise AI budget resources. Enabling this changes organization billing policy and requires suitable Billing permissions."
  default     = false
}

variable "team_name" {
  type        = string
  description = "Name for the team managed by this local development module."
  default     = "Terraform Provider Dev Loop"
}

variable "team_membership_user_id" {
  type        = string
  description = "Existing user ID to add to the managed team. The user must not belong to another team. Leave unset to avoid moving a real user."
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
