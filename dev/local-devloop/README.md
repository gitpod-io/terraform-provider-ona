# Local Dev Loop Module

This module exercises all Terraform provider resources and both runner data sources:

- `ona_runner.devloop`
- `ona_service_account.devloop`
- `ona_group.devloop`
- `ona_group_membership.devloop`
- `ona_organization_role_assignment.devloop`
- `ona_environment_class.devloop`
- `ona_project.devloop`
- `ona_webhook.devloop`
- `ona_warm_pool.devloop`
- `ona_scm_integration.github_oauth`
- `ona_scm_integration.gitlab_pat`
- `ona_scm_integration.azuredevops_entra`
- `ona_scm_integration.azuredevops_server`
- `ephemeral.ona_runner_token.devloop`
- `ephemeral.ona_webhook_secret.devloop`
- `data.ona_runners.all`
- `data.ona_runner.devloop`
- `data.ona_warm_pool.devloop`
- `data.ona_warm_pools.devloop`

Build the provider and configure Terraform to use the local binary:

```shell
mkdir -p .bin
go build -o .bin/terraform-provider-ona .
cat > terraformrc <<EOF
provider_installation {
  dev_overrides {
    "gitpod-io/ona" = "${PWD}/.bin"
    "ona-com/ona"  = "${PWD}/.bin"
  }
  direct {}
}
EOF
```

Run the default plan:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop plan -input=false
```

To create the resources and read the runner back through the singular data
source, run:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop apply -auto-approve -input=false
```

The apply output includes `cloudformation_template_url` for AWS EC2 runners,
`managed_service_account_id` for the managed service account, and the managed
warm pool ID. Runner registration tokens are consumed through
`ephemeral.ona_runner_token` during apply, so they are not written as normal
Terraform outputs or stored in state.

Webhook creation requires a user or administrator token; the Ona API rejects
service-account credentials for this operation. The dev loop retrieves the
generated signing secret through `ephemeral.ona_webhook_secret` and writes it
to `/tmp/ona-webhook-secret.txt` through an ephemeral module input, so the
secret is not stored in Terraform plan or state. Change
`webhook_secret_version` to rotate the secret and refresh the local file:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop apply \
  -var='webhook_secret_version=v2' \
  -auto-approve -input=false
```

Removing `ona_webhook.devloop` deletes the remote webhook. If workflows are
bound to it, Ona converts their webhook triggers to manual triggers.

The dev loop passes the token from the ephemeral resource into an ephemeral
module input:

```hcl
ephemeral "ona_runner_token" "devloop" {
  runner_id = ona_runner.devloop.runner_id
}

module "token_writer" {
  source = "./modules/token-writer"

  runner_token = ephemeral.ona_runner_token.devloop.token
}
```

Clean up the resources afterward with:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop destroy -auto-approve -input=false
rm -f /tmp/ona-runner-token.txt /tmp/ona-webhook-secret.txt
```
