resource "ona_project" "api" {
  name                 = "acme-api"
  repository_clone_url = "https://github.com/acme/api.git"
  branch               = "main"

  environment_class {
    environment_class_id = ona_environment_class.large.id
    order                = 0
  }

  prebuild_configuration {
    enabled               = true
    environment_class_ids = [ona_environment_class.large.id]
    timeout               = "1h"

    daily_schedule {
      hour_utc = 5
    }
  }
}

resource "ona_warm_pool" "api_large" {
  project_id           = ona_project.api.id
  environment_class_id = ona_environment_class.large.id
  min_size             = 0
  max_size             = 5
}
