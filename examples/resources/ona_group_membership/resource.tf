resource "ona_group" "runner_admins" {
  name        = "Runner Admins"
  description = "Users and service accounts that administer runners."
}

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

resource "ona_group_membership" "runner_admins" {
  for_each = data.ona_user.runner_admins

  group_id = ona_group.runner_admins.id
  user_id  = each.value.user_id
}

resource "ona_service_account" "runner_admin_automation" {
  name        = "runner-admin-automation"
  description = "Runner administration automation"
  valid_until = "2099-01-01T00:00:00Z"
}

resource "ona_group_membership" "runner_admin_automation" {
  group_id           = ona_group.runner_admins.id
  service_account_id = ona_service_account.runner_admin_automation.id
}
