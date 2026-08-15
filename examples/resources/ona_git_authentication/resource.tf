variable "github_service_account_pat" {
  description = "GitHub personal access token for the automation service account."
  type        = string
  sensitive   = true
  ephemeral   = true
}

resource "ona_group" "automation_executors" {
  name        = "Automation Executors"
  description = "Service accounts that run automations."
}

resource "ona_group_membership" "automation_executor" {
  group_id           = ona_group.automation_executors.id
  service_account_id = ona_service_account.automation.id
}

resource "ona_runner_role_assignment" "automation_executor" {
  runner_id = ona_scm_integration.github_pat.runner_id
  group_id  = ona_group.automation_executors.id
  role      = "user"
}

resource "ona_git_authentication" "automation" {
  service_account_id            = ona_service_account.automation.id
  scm_integration_id            = ona_scm_integration.github_pat.id
  personal_access_token         = var.github_service_account_pat
  personal_access_token_version = "2026-07-28"

  depends_on = [
    ona_group_membership.automation_executor,
    ona_runner_role_assignment.automation_executor,
  ]
}
