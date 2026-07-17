data "ona_user" "existing" {
  user_id = "<user-id>"
}

output "existing_user_id" {
  value = data.ona_user.existing.user_id
}
