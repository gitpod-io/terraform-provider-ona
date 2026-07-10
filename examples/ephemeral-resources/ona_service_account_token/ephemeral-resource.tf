ephemeral "ona_service_account_token" "ci" {
  service_account_id = ona_service_account.ci.id
  description        = "GitHub Actions"
  valid_for          = "2160h"
}
