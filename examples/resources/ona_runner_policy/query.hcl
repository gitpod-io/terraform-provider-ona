variable "runner_id" {
  type = string
}

list "ona_runner_policy" "runner" {
  provider         = ona
  include_resource = true

  config {
    runner_ids = [var.runner_id]
  }
}
