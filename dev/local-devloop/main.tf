terraform {
  required_providers {
    ona = {
      source = "registry.terraform.io/gitpod-io/ona"
    }
  }
}

provider "ona" {
  host  = var.ona_host
  token = var.ona_token
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
