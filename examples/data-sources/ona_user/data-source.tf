locals {
  runner_admins = {
    alice = { email = "alice@example.com", login_provider = "github" }
    bob   = { email = "bob@example.com", login_provider = "google" }
    carol = { email = "carol@example.com", login_provider = "custom" }
  }
}

data "ona_user" "runner_admins" {
  for_each = local.runner_admins

  email          = each.value.email
  login_provider = each.value.login_provider
}

output "runner_admin_user_ids" {
  value = {
    for name, user in data.ona_user.runner_admins : name => user.user_id
  }
}
