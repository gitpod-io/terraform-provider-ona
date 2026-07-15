resource "ona_scm_integration" "github_oauth" {
  runner_id = ona_runner.aws_primary.runner_id

  kind = "github"
  host = "github.com"

  auth_mode                   = "oauth"
  oauth_client_id             = var.github_oauth_client_id
  oauth_client_secret         = var.github_oauth_client_secret
  oauth_client_secret_version = "2026-06-24"
}

resource "ona_scm_integration" "github_pat" {
  runner_id = ona_runner.aws_primary.runner_id

  kind      = "github"
  host      = "github.com"
  auth_mode = "pat"
}

resource "ona_scm_integration" "azure_devops_entra" {
  runner_id = ona_runner.aws_primary.runner_id

  kind = "azuredevops_entra"
  host = "dev.azure.com/acme"

  auth_mode                   = "oauth"
  oauth_client_id             = var.azure_devops_oauth_client_id
  oauth_client_secret         = var.azure_devops_oauth_client_secret
  oauth_client_secret_version = "2026-06-24"
  issuer_url                  = "https://login.microsoftonline.com/00000000-0000-0000-0000-000000000000/v2.0"
}

resource "ona_scm_integration" "azure_devops_server" {
  runner_id = ona_runner.aws_primary.runner_id

  kind              = "azuredevops_server"
  host              = "ado.example.com"
  auth_mode         = "pat"
  virtual_directory = "/tfs"
}
