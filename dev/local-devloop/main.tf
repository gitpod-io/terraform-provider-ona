terraform {
  required_providers {
    ona = {
      source = "gitpod-io/ona"
    }
  }
}

provider "ona" {
  host  = var.ona_host
  token = var.ona_token
}

resource "ona_service_account" "devloop" {
  name        = var.service_account_name
  description = "Service account created by the Terraform provider local dev loop."
  valid_until = var.service_account_valid_until
}

resource "ona_group" "devloop" {
  name        = var.group_name
  description = "Group created by the Terraform provider local dev loop."
}

resource "ona_group" "runner_sharing" {
  count = var.runner_sharing_role == null ? 0 : 1

  name        = "${var.group_name} Runner Sharing"
  description = "Empty group created to exercise direct runner sharing without organization-derived access."
}

resource "ona_team" "devloop" {
  name = var.team_name
}

resource "ona_team_membership" "devloop" {
  count = var.team_membership_user_id == null ? 0 : 1

  team_id = ona_team.devloop.id
  user_id = var.team_membership_user_id
}

resource "ona_group_membership" "devloop" {
  group_id           = ona_group.devloop.id
  service_account_id = ona_service_account.devloop.id
}

resource "ona_organization_role_assignment" "devloop" {
  for_each = toset([
    "organization_admin",
    "runners_admin",
    "projects_admin",
  ])

  group_id = ona_group.devloop.id
  role     = each.value
}

resource "ona_automation_role_assignment" "devloop" {
  count = var.automation_sharing_automation_id == null ? 0 : 1

  automation_id = var.automation_sharing_automation_id
  group_id      = ona_group.devloop.id
  role          = "executor"
}

resource "ona_organization_ai_budget" "credits" {
  count = var.enable_ai_budgets ? 1 : 0

  mode                 = "credits"
  monthly_credit_limit = var.organization_monthly_credit_limit
}

resource "ona_organization_ai_budget" "byok" {
  count = var.enable_ai_budgets ? 1 : 0

  mode                          = "byok"
  monthly_cost_limit_microunits = var.organization_monthly_cost_limit_microunits
  currency                      = "usd"
}

resource "ona_user_ai_budget" "service_account_credits" {
  count = var.enable_ai_budgets ? 1 : 0

  user_id              = ona_service_account.devloop.id
  mode                 = "credits"
  monthly_credit_limit = var.service_account_monthly_credit_limit
}

resource "ona_user_ai_budget" "service_account_byok_exemption" {
  count = var.enable_ai_budgets ? 1 : 0

  user_id = ona_service_account.devloop.id
  mode    = "byok"
  no_cap  = true
}

resource "ona_team_ai_budget" "credits" {
  count = var.enable_ai_budgets ? 1 : 0

  team_id       = ona_team.devloop.id
  mode          = "credits"
  credit_budget = var.team_credit_budget
}

resource "ona_team_ai_budget" "byok" {
  count = var.enable_ai_budgets ? 1 : 0

  team_id                = ona_team.devloop.id
  mode                   = "byok"
  cost_budget_microunits = var.team_cost_budget_microunits
  cost_budget_currency   = "usd"
}

resource "ona_runner" "devloop" {
  name            = var.runner_name
  runner_provider = var.runner_provider

  configuration {
    region                           = var.runner_region
    release_channel                  = var.release_channel
    auto_update                      = var.auto_update
    devcontainer_image_cache_enabled = var.devcontainer_image_cache_enabled
    log_level                        = var.log_level

    update_window {
      start = "02:00"
      end   = "04:00"
    }
  }
}

data "ona_runners" "all" {
  depends_on = [ona_runner.devloop]
}

data "ona_runner" "devloop" {
  runner_id = ona_runner.devloop.runner_id
}

resource "ona_runner_token" "devloop" {
  runner_id     = ona_runner.devloop.runner_id
  token_version = var.runner_token_version
}

resource "ona_runner_role_assignment" "devloop" {
  count = var.runner_sharing_role == null ? 0 : 1

  runner_id = ona_runner.devloop.runner_id
  group_id  = ona_group.runner_sharing[0].id
  role      = var.runner_sharing_role
}

resource "ona_environment_class" "devloop" {
  runner_id = ona_runner.devloop.runner_id

  display_name = "Dev Loop"
  description  = "Environment class created by the Terraform provider local dev loop."
  enabled      = true

  configuration = {
    machineType = "m6i.large"
    diskSizeGb  = "100"
  }
}

resource "ona_project" "devloop" {
  name                 = "terraform-provider-devloop"
  repository_clone_url = "https://github.com/gitpod-io/gitpod-next.git"
  branch               = "main"

  devcontainer_file_path = ".devcontainer/devcontainer.json"
  automations_file_path  = ".ona/automations.yaml"

  environment_class {
    environment_class_id = ona_environment_class.devloop.id
    order                = 0
  }

  prebuild_configuration {
    enabled               = true
    environment_class_ids = [ona_environment_class.devloop.id]
    timeout               = "1h"

    daily_schedule {
      hour_utc = 5
    }
  }
}

resource "ona_project_role_assignment" "devloop" {
  project_id = ona_project.devloop.id
  group_id   = ona_group.devloop.id
  role       = var.project_sharing_role

  depends_on = [ona_organization_role_assignment.devloop]
}

resource "ona_webhook" "devloop" {
  name           = "Terraform Provider Dev Loop"
  description    = "Webhook created by the Terraform provider local dev loop."
  type           = "repository"
  scm_provider   = "github"
  secret_version = var.webhook_secret_version

  repository_scopes = [
    {
      host  = "github.com"
      owner = "gitpod-io"
      name  = "terraform-provider-ona"
    }
  ]
}

ephemeral "ona_webhook_secret" "devloop" {
  webhook_id = ona_webhook.devloop.id
}

module "webhook_secret_writer" {
  source = "./modules/webhook-secret-writer"

  webhook_secret         = ephemeral.ona_webhook_secret.devloop.secret
  webhook_secret_version = var.webhook_secret_version
}

resource "ona_warm_pool" "devloop" {
  project_id           = ona_project.devloop.id
  environment_class_id = ona_environment_class.devloop.id
  min_size             = 0
  max_size             = 1
}

data "ona_warm_pool" "devloop" {
  warm_pool_id = ona_warm_pool.devloop.id
}

data "ona_warm_pools" "devloop" {
  project_ids           = [ona_project.devloop.id]
  environment_class_ids = [ona_environment_class.devloop.id]
}

moved {
  from = ona_scm_integration.github_oauth
  to   = ona_scm_integration.github_pat
}

resource "ona_scm_integration" "github_pat" {
  runner_id = ona_runner.devloop.runner_id

  kind = "github"
  host = "github.com"

  auth_mode = "pat"
}

resource "ona_git_authentication" "devloop" {
  count = var.enable_git_authentication ? 1 : 0

  service_account_id            = ona_service_account.devloop.id
  scm_integration_id            = ona_scm_integration.github_pat.id
  personal_access_token         = var.git_personal_access_token
  personal_access_token_version = var.git_personal_access_token_version

  depends_on = [
    ona_group_membership.devloop,
    ona_organization_role_assignment.devloop,
  ]
}

resource "ona_scm_integration" "gitlab_pat" {
  runner_id = ona_runner.devloop.runner_id

  kind = "gitlab"
  host = "gitlab.com"

  auth_mode = "pat"
}

resource "ona_scm_integration" "azuredevops_entra" {
  runner_id = ona_runner.devloop.runner_id

  kind = "azuredevops_entra"
  host = "dev.azure.com"

  auth_mode = "pat"
}

resource "ona_scm_integration" "azuredevops_server" {
  runner_id = ona_runner.devloop.runner_id

  kind = "azuredevops_server"
  host = "azuredevops.example.com"

  auth_mode         = "pat"
  virtual_directory = "/tfs"
}
