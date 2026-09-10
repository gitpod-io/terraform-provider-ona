variable "github_app_id" {
  type        = string
  description = "App ID of an existing organization-owned GitHub App."
}

variable "github_app_credentials" {
  type = object({
    private_key    = string
    client_secret  = string
    webhook_secret = string
  })
  description = "Complete credential bundle for creation or rotation. Omit when importing."
  sensitive   = true
  ephemeral   = true
  default     = null
}

variable "github_app_credentials_version" {
  type        = number
  description = "Set to 1 for creation; change to a new positive integer with the full bundle for rotation. Omit when importing."
  default     = null
}

resource "ona_github_app_integration" "company" {
  app_id              = var.github_app_id
  credentials         = var.github_app_credentials
  credentials_version = var.github_app_credentials_version
}

output "github_app_integration_id" {
  value = ona_github_app_integration.company.id
}

output "github_app_setup" {
  value = ona_github_app_integration.company.setup
}

output "github_app_installation" {
  value = ona_github_app_integration.company.installation
}
