variable "organization_id" {
  type = string
}

list "ona_security_policy" "organization" {
  provider         = ona
  include_resource = true

  config {
    organization_id = var.organization_id
  }
}
