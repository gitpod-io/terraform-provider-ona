resource "ona_scm_integration" "github" {
  runner_id = ona_runner.aws_primary.runner_id

  scm_id = "github"
  host   = "github.com"

  auth_mode                   = "oauth"
  oauth_client_id             = var.github_oauth_client_id
  oauth_client_secret         = var.github_oauth_client_secret
  oauth_client_secret_version = "2026-06-24"
}
