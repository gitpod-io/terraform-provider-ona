resource "ona_oidc_config" "org" {
  custom_claim_fields = [
    "project_id",
    "creator_email",
  ]
}
