resource "ona_webhook" "deployments" {
  name           = "Deployment events"
  description    = "Receives deployment events for the platform repository."
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

resource "ona_webhook" "organization" {
  name           = "Organization events"
  description    = "Receives events for all repositories in the organization."
  type           = "organization"
  scm_provider   = "github"
  secret_version = "v1"

  organization_scope = {
    host = "github.com"
    name = "example-organization"
  }
}
