resource "ona_oidc_config" "org" {
  version = "v3"

  extra_sub_fields = [
    "project_id",
    "creator_email",
  ]
}
