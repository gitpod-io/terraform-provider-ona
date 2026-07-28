variable "github_service_account_pat" {
  description = "GitHub personal access token for the automation service account."
  type        = string
  sensitive   = true
  ephemeral   = true
}

resource "ona_git_authentication" "automation" {
  service_account_id            = ona_service_account.automation.id
  scm_integration_id            = ona_scm_integration.github_pat.id
  personal_access_token         = var.github_service_account_pat
  personal_access_token_version = "2026-07-28"
}
