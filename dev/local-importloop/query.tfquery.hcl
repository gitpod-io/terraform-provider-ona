list "ona_announcement_banner" "all" {
  provider         = ona
  include_resource = true
}

list "ona_automation" "all" {
  provider         = ona
  include_resource = true
}

list "ona_integration" "all" {
  provider         = ona
  include_resource = true
}

list "ona_runner" "all" {
  provider         = ona
  include_resource = true
}

list "ona_runner_llm_integration" "all" {
  provider         = ona
  include_resource = true
}

list "ona_scm_integration" "all" {
  provider         = ona
  include_resource = true
}

list "ona_git_authentication" "all" {
  provider         = ona
  include_resource = true
}

list "ona_environment_class" "all" {
  provider         = ona
  include_resource = true
}

list "ona_group_membership" "all" {
  provider         = ona
  include_resource = true

  config {
    group_id = var.group_membership_group_id
  }
}

list "ona_team_membership" "all" {
  provider         = ona
  include_resource = true

  config {
    team_id = var.team_membership_team_id
  }
}

list "ona_custom_domain" "all" {
  provider         = ona
  include_resource = true
}

list "ona_group" "all" {
  provider         = ona
  include_resource = true
}

list "ona_team" "all" {
  provider         = ona
  include_resource = true
}

list "ona_sso_configuration" "all" {
  provider         = ona
  include_resource = true
}

list "ona_terms_of_service" "all" {
  provider         = ona
  include_resource = true
}

list "ona_oidc_config" "all" {
  provider         = ona
  include_resource = true
}

list "ona_organization_policies" "all" {
  provider         = ona
  include_resource = true
}

list "ona_organization_role_assignment" "all" {
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

list "ona_webhook" "all" {
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

list "ona_skill" "all" {
  provider         = ona
  include_resource = true
}
