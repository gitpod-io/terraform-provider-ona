data "ona_users" "active_members" {
  search   = "example.com"
  statuses = ["active"]
  roles    = ["member"]
}

locals {
  active_members_by_id = {
    for user in data.ona_users.active_members.users : user.user_id => user
  }
}
