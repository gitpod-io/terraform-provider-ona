list "ona_group_membership" "all" {
  provider         = ona
  include_resource = true
  config { group_id = "<group-id>" }
}
