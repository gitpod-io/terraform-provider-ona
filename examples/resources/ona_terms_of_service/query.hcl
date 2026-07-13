variable "organization_id" { type = string }

list "ona_terms_of_service" "organization" {
  provider         = ona
  include_resource = true
  config { organization_id = var.organization_id }
}
