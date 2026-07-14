variable "scope" {
  type = string
}

variable "owner_id" {
  type    = string
  default = null
}
list "ona_secret" "scope" {
  provider = ona
  include_resource = true
  config {
    scope = var.scope
    project_id = var.scope == "project" ? var.owner_id : null
    user_id = var.scope == "user" ? var.owner_id : null
    service_account_id = var.scope == "service_account" ? var.owner_id : null
  }
}
