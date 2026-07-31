variable "group_id" { type = string }
list "ona_group_membership" "group" {
  provider = ona
  include_resource = true
  config { group_id = var.group_id }
}
