resource "ona_sso_configuration" "corp" {
  display_name = "Acme Okta"
  issuer_url   = "https://acme.okta.com"
  client_id    = var.oidc_client_id

  client_secret         = var.oidc_client_secret
  client_secret_version = var.oidc_client_secret_version

  email_domains     = ["acme.com"]
  additional_scopes = ["groups"]
  claims_expression = "claims.email_verified"
  state             = "active"
}
