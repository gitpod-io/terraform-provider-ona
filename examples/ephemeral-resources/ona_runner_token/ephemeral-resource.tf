ephemeral "ona_runner_token" "aws_primary" {
  runner_id = ona_runner.aws_primary.runner_id
}

module "runner_bootstrap" {
  source = "./modules/runner-bootstrap"

  # The child module variable should be sensitive and ephemeral so the runner
  # registration token is available during apply without being written to state.
  runner_token = ephemeral.ona_runner_token.aws_primary.token
}
