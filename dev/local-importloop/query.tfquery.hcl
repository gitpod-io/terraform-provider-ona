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

list "ona_secret" "all" {
  provider         = ona
  include_resource = true

  config {
    scope = "organization"
  }
}
