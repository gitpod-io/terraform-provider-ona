variable "runner_id" {
  type        = string
  default     = null
  description = "Optional runner ID used to scope policy discovery."
}

list "ona_runner_policy" "runner" {
  provider         = ona
  include_resource = true

  config {
    runner_ids = var.runner_id == null ? [] : [var.runner_id]
  }
}
