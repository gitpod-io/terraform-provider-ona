resource "ona_project_insights" "api" {
  project_id = ona_project.api.id
  enabled    = true
}
