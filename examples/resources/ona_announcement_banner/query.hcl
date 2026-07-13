variable "organization_id" {
  type = string
}

list "ona_announcement_banner" "organization" {
  provider         = ona
  include_resource = true

  config {
    organization_id = var.organization_id
  }
}
