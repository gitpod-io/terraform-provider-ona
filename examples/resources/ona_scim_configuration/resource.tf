resource "ona_scim_configuration" "corp" {
  sso_configuration_id = ona_sso_configuration.corp.id
  name                 = "Acme SCIM"
  enabled              = true

  token_expires_in = "8760h"

  allow_unverified_email_account_linking = false
}
