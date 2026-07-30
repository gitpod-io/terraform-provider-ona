list "ona_runner" "all" {
  provider         = ona
  include_resource = true
}

list "ona_scm_integration" "all" {
  provider         = ona
  include_resource = true
}

list "ona_environment_class" "all" {
  provider         = ona
  include_resource = true
}

list "ona_custom_domain" "all" {
  provider         = ona
  include_resource = true
}

list "ona_sso_configuration" "all" {
  provider         = ona
  include_resource = true
}

list "ona_oidc_config" "all" {
  provider         = ona
  include_resource = true
}

list "ona_project" "all" {
  provider         = ona
  include_resource = true
}

list "ona_warm_pool" "all" {
  provider         = ona
  include_resource = true
}

list "ona_scim_configuration" "all" {
  provider         = ona
  include_resource = true
}

list "ona_security_policy" "all" {
  provider         = ona
  include_resource = true
}

list "ona_secret" "all" {
  provider         = ona
  include_resource = true

  config {
    scope = "organization"
  }
}

list "ona_service_account" "all" {
  provider         = ona
  include_resource = true
}
