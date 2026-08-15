variable "team_id" { type = string }

list "ona_team_membership" "team" {
  provider         = ona
  include_resource = true
  config { team_id = var.team_id }
}
