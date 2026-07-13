variable "organization_id" { type = string }

list "ona_custom_domain" "organization" {
  provider         = ona
  include_resource = true
  config { organization_id = var.organization_id }
}
