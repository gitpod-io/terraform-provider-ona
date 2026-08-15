list "ona_team_membership" "all" {
  provider         = ona
  include_resource = true
  config { team_id = "<team-id>" }
}
