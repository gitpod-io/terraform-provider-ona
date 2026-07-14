resource "ona_project" "api" {
  name                 = "acme-api"
  repository_clone_url = "https://github.com/acme/api.git"
  branch               = "main"

  devcontainer_file_path = ".devcontainer/devcontainer.json"
  automations_file_path  = ".ona/automations.yaml"

  environment_class {
    environment_class_id = ona_environment_class.large.id
    order                = 0
  }

  environment_class {
    local_runner = true
    order        = 1
  }

  prebuild_configuration {
    enabled                 = true
    environment_class_ids   = [ona_environment_class.large.id]
    timeout                 = "1h"
    enable_jetbrains_warmup = true

    daily_schedule {
      hour_utc = 5
    }

    executor {
      id        = ona_service_account.terraform.id
      principal = "service_account"
    }
  }
}
