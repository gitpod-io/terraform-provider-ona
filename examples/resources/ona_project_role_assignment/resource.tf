variable "project_id" {
  type        = string
  description = "ID of an existing Ona project."
}

resource "ona_group" "project_editors" {
  name        = "Project Editors"
  description = "People allowed to edit the project configuration."
}

resource "ona_project_role_assignment" "editors" {
  project_id = var.project_id
  group_id   = ona_group.project_editors.id
  role       = "editor"
}
