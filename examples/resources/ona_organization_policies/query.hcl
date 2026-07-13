variable "organization_id" {
  type = string
}

list "ona_organization_policies" "organization" {
  provider         = ona
  include_resource = true

  config {
    organization_id = var.organization_id
  }
}
