variable "runner_id" {
  type = string
}

list "ona_scm_integration" "runner" {
  provider         = ona
  include_resource = true

  config {
    runner_ids = [var.runner_id]
  }
}
