list "ona_secret" "organization" {
  provider         = ona
  include_resource = true

  config {
    scope = "organization"
  }
}
