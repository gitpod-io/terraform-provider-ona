resource "ona_runner_token" "bootstrap" {
  runner_id     = ona_runner.example.runner_id
  token_version = "v1"
}

module "runner_bootstrap" {
  source = "./modules/runner-bootstrap"

  runner_token = ona_runner_token.bootstrap.token
}
