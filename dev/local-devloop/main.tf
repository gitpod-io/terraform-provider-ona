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

ephemeral "ona_runner_token" "devloop" {
  runner_id = ona_runner.devloop.runner_id
}

module "token_writer" {
  source = "./modules/token-writer"

  # Pass the runner token through an ephemeral input so Terraform can use it during apply
  # without writing the token to plan or state.
  runner_token = ephemeral.ona_runner_token.devloop.token
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

resource "ona_scm_integration" "github_oauth" {
  runner_id = ona_runner.devloop.runner_id

  scm_id = "github"
  host   = "github.com"

  auth_mode = "pat"
}

resource "ona_scm_integration" "gitlab_pat" {
  runner_id = ona_runner.devloop.runner_id

  scm_id = "gitlab"
  host   = "gitlab.com"

  auth_mode = "pat"
}

resource "ona_scm_integration" "azuredevops_entra" {
  runner_id = ona_runner.devloop.runner_id

  scm_id = "azuredevops_entra"
  host   = "dev.azure.com"

  auth_mode = "pat"
}

resource "ona_scm_integration" "azuredevops_server" {
  runner_id = ona_runner.devloop.runner_id

  scm_id = "azuredevops_server"
  host   = "azuredevops.example.com"

  auth_mode         = "pat"
  virtual_directory = "/tfs"
}

data "ona_integration_definitions" "available" {}

locals {
  linear_integration_definition = one([
    for definition in data.ona_integration_definitions.available.definitions : definition
    if definition.host == "linear.app"
  ])
}

resource "ona_integration" "linear" {
  integration_definition_id = local.linear_integration_definition.id
  enabled                   = true
}
