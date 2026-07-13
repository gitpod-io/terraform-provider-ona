resource "ona_runner_policy" "org_members" {
  runner_id = ona_runner.example.runner_id
  group_id  = ona_group.example.id
  role      = "user"
}
