resource "ona_webhook" "deployments" {
  name           = "Deployment events"
  type           = "repository"
  scm_provider   = "github"
  secret_version = "v1"

  repository_scopes = [
    {
      host  = "github.com"
      owner = "example-organization"
      name  = "platform"
    }
  ]
}

ephemeral "ona_webhook_secret" "deployments" {
  webhook_id = ona_webhook.deployments.id
}

module "webhook_secret_target" {
  source = "./modules/webhook-secret-target"

  # Write the secret to an external secret target or SCM configuration without
  # persisting it in Terraform state. Change ona_webhook.deployments.secret_version
  # to rotate the secret before this value is retrieved during apply.
  webhook_secret = ephemeral.ona_webhook_secret.deployments.secret
}
