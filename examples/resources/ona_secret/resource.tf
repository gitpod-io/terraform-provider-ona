variable "secret_value" {
  type      = string
  sensitive = true
}

resource "ona_secret" "organization_api_key" {
  scope = "organization"
  name  = "THIRD_PARTY_API_KEY"

  value         = var.secret_value
  value_version = "2026-07-10"

  environment_variable = true
}

variable "registry_username" {
  type = string
}

variable "registry_password" {
  type      = string
  sensitive = true
}

resource "ona_secret" "registry" {
  scope      = "project"
  project_id = ona_project.api.id
  name       = "PRIVATE_REGISTRY_AUTH"

  value         = base64encode("${var.registry_username}:${var.registry_password}")
  value_version = "2026-07-10"

  container_registry_basic_auth_host = "registry.example.com"
}

resource "ona_secret" "ssh_config" {
  scope     = "user"
  user_id   = "00000000-0000-0000-0000-000000000000"
  name      = "SSH_CONFIG"
  value     = var.secret_value
  file_path = "/home/ona/.ssh/config"
}

resource "ona_secret" "automation_token" {
  scope              = "service_account"
  service_account_id = ona_service_account.terraform.id
  name               = "AUTOMATION_TOKEN"

  value         = var.secret_value
  value_version = "2026-07-10"

  api_only = true

  credential_proxy {
    target_hosts = ["api.example.com"]
    header       = "Authorization"
  }
}
