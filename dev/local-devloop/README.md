# Local Dev Loop Module

This module exercises the Terraform provider resources, ephemeral resources,
and data sources:

- `ona_runner.devloop`
- `ona_service_account.devloop`
- `ona_group.devloop`
- `ona_team.devloop`
- `ona_group_membership.devloop`
- `ona_organization_role_assignment.devloop`
- `ona_organization_ai_budget.credits`
- `ona_organization_ai_budget.byok`
- `ona_user_ai_budget.service_account_credits`
- `ona_user_ai_budget.service_account_byok_exemption`
- `ona_team_ai_budget.credits`
- `ona_team_ai_budget.byok`
- `ona_environment_class.devloop`
- `ona_project.devloop`
- `ona_webhook.devloop`
- `ona_warm_pool.devloop`
- `ona_scm_integration.github_oauth`
- `ona_scm_integration.gitlab_pat`
- `ona_scm_integration.azuredevops_entra`
- `ona_scm_integration.azuredevops_server`
- `ona_integration.linear`
- `ephemeral.ona_runner_token.devloop`
- `ephemeral.ona_webhook_secret.devloop`
- `data.ona_integration_definitions.available`
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
`managed_service_account_id` and `managed_team_id`, the managed warm pool and
integration IDs, and the number of visible integration definitions. Runner
registration tokens are consumed through
`ephemeral.ona_runner_token` during apply, so they are not written as normal
Terraform outputs or stored in state.

The integration uses the visible built-in definition for `linear.app`, so the
dev loop does not require or persist an OAuth client secret.

AI budget resources are opt-in because they require an enterprise organization,
suitable Billing permissions, and change organization billing policy. Enabling
them exercises organization, service-account, and team budgets, including
separate credit and BYOK resources against the shared `ona_team.devloop`
allocation:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop apply \
  -var='enable_ai_budgets=true' \
  -auto-approve -input=false
```

Both team resources may report the same allocation ID. Updating or destroying
one mode preserves the complementary mode. Destroy the devloop with the same
variable values so Terraform can remove every enabled budget resource.

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
