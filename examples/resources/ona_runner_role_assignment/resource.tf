variable "runner_id" {
  type        = string
  description = "ID of an existing Ona runner."
}

resource "ona_group" "runner_admins" {
  name        = "Runner Admins"
  description = "People allowed to configure and share the runner."
}

resource "ona_runner_role_assignment" "admins" {
  runner_id = var.runner_id
  group_id  = ona_group.runner_admins.id
  role      = "admin"
}
