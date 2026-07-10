resource "ona_group" "terraform_admins" {
  name        = "Terraform Admins"
  description = "Service accounts that administer Terraform-managed organization settings."
}

resource "ona_service_account" "terraform" {
  name        = "terraform"
  description = "Terraform organization automation"
  valid_until = "2099-01-01T00:00:00Z"
}

resource "ona_group_membership" "terraform_service_account" {
  group_id           = ona_group.terraform_admins.id
  service_account_id = ona_service_account.terraform.id
}
