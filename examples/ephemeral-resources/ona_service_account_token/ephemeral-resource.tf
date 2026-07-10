resource "ona_service_account" "terraform" {
  name        = "terraform"
  description = "Terraform automation"
  valid_until = "2099-01-01T00:00:00Z"
}

ephemeral "ona_service_account_token" "terraform" {
  service_account_id = ona_service_account.terraform.id
  description        = "Terraform automation"
  valid_for          = "2160h"
}

module "terraform_token_secret" {
  source = "./modules/service-account-token-secret"

  # Use a child module that writes to an external secret target through an
  # ephemeral variable or write-only argument. Use the stored token as ONA_TOKEN
  # for later Terraform runs.
  service_account_token = ephemeral.ona_service_account_token.terraform.token
}
