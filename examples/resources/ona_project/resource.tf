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
}
