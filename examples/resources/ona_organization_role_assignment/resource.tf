resource "ona_group" "terraform_admins" {
  name        = "Terraform Admins"
  description = "Service accounts that administer Terraform-managed organization settings."
}

resource "ona_organization_role_assignment" "terraform_admin" {
  group_id = ona_group.terraform_admins.id
  role     = "organization_admin"
}
