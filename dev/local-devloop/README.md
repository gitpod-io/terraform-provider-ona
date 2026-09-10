# Local Dev Loop Module

This module exercises the Terraform provider resources, ephemeral resources,
and data sources:

- `ona_runner.devloop`
- `ona_service_account.devloop`
- `ona_group.devloop`
- `ona_group.runner_sharing[0]` (opt-in)
- `ona_team.devloop`
- `ona_team_membership.devloop[0]` (opt-in)
- `ona_group_membership.devloop`
- `ona_organization_role_assignment.devloop`
- `ona_automation_role_assignment.devloop[0]` (opt-in)
- `ona_organization_ai_budget.credits`
- `ona_organization_ai_budget.byok`
- `ona_user_ai_budget.service_account_credits`
- `ona_user_ai_budget.service_account_byok_exemption`
- `ona_team_ai_budget.credits`
- `ona_team_ai_budget.byok`
- `ona_environment_class.devloop`
- `ona_runner_role_assignment.devloop[0]` (opt-in)
- `ona_project.devloop`
- `ona_project_role_assignment.devloop`
- `ona_webhook.devloop`
- `ona_warm_pool.devloop`
- `ona_scm_integration.github_pat`
- `ona_github_app_integration.devloop[0]` (opt-in)
- `ona_git_authentication.devloop` (opt-in)
- `ona_scm_integration.gitlab_pat`
- `ona_scm_integration.azuredevops_entra`
- `ona_scm_integration.azuredevops_server`
- `ona_integration.linear`
- `ona_runner_token.devloop`
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

The apply output includes `terraform_module_url` for the selected runner cloud
provider, `cloudformation_template_url` for AWS EC2 runners,
`managed_runner_token`, `managed_runner_role_assignment_id` when runner sharing
is enabled, `managed_service_account_id`, `managed_team_id`, the managed project
role assignment, warm pool and integration IDs, and the number of visible
integration definitions. Runner registration tokens are sensitive but stored
in Terraform state so normal resources and module inputs can consume them. Use
an encrypted, access-controlled remote backend. The token is valid for 24 hours
and can only be used once; change `runner_token_version` deliberately when the
local consumer needs a fresh token. Terraform redacts `managed_runner_token`
from normal output; use `terraform output -raw managed_runner_token` when a
local downstream consumer needs its value.

Team membership is opt-in because an Ona user can belong to only one team in an
organization. Only set `team_membership_user_id` to an existing user who is not
assigned to another team:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop apply \
  -var='team_membership_user_id=<user-id>' \
  -auto-approve -input=false
```

Keep the same variable value when destroying the dev loop. Moving a user to a
different team removes the old membership before creating the new one; do not
set `create_before_destroy` for `ona_team_membership`.

Rotate the runner token by changing the root rotation marker:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop apply \
  -var='runner_token_version=v2' \
  -auto-approve -input=false
```

The integration uses the visible built-in definition for `linear.app`, so the
dev loop does not require or persist an OAuth client secret.

## Test an organization-managed GitHub App

This resource is disabled by default. Use Terraform 1.14 or later and an
existing disposable organization-owned GitHub App. A GitHub administrator
registers the App, selects its permissions and events, and obtains its App ID
and three credentials. An Ona organization administrator must be able to use
the preview **Custom GitHub App** browser workflow, with a GitHub user who can
approve installation and select repositories. See the
[resource guide](../../templates/resources/github_app_integration.md.tmpl)
for setup, rotation, and adoption details.

The provider identifies custom Apps using the dashboard's
[metadata comparison](../../templates/resources/github_app_integration.md.tmpl#app-classification).
Use an App whose ID, slug, or client ID differs from the shared definition.

Use the local provider override above and supply `ONA_TOKEN` through your
normal credential source. Set `TF_VAR_github_app_credentials` to a JSON object
with `private_key`, `client_secret`, and `webhook_secret` through a secret-store
integration or a masked runtime input. The variable is ephemeral and sensitive,
and the resource consumes it through a write-only argument, so the bundle is
not stored in Terraform plan or state. Set the remaining non-secret inputs:

```shell
export TF_CLI_CONFIG_FILE="${PWD}/terraformrc"
export TF_VAR_enable_github_app_integration=true
export TF_VAR_github_app_id='<github-app-id>'
export TF_VAR_github_app_credentials_version=1
github_app_target='ona_github_app_integration.devloop[0]'

terraform -chdir=dev/local-devloop apply \
  -target="${github_app_target}" -input=false
```

The outputs include `managed_github_app_integration_id`, `github_app_setup`,
and `github_app_installation`. Use the returned callback and webhook URLs in
the GitHub App settings, set the same webhook secret on GitHub, and enable
**Request user authorization (OAuth) during installation**. The callback may
be on Ona's canonical host even when the provider uses a custom domain. Open
`github_app_setup.installation_url` in an authenticated browser and select the
same Ona organization that Terraform manages to install or connect the App.
The stable URL ends in `/settings/org-integrations`; the browser handles the
custom-domain handoff.
Refresh after installation to observe its ID and account metadata:

```shell
terraform -chdir=dev/local-devloop apply \
  -refresh-only -target="${github_app_target}" -input=false
terraform -chdir=dev/local-devloop plan \
  -target="${github_app_target}" -input=false
```

The second command should produce an empty plan. For same-App rotation,
coordinate the secret changes in GitHub, supply the complete replacement
bundle, and change `TF_VAR_github_app_credentials_version` to `2` before
applying. Secret changes without a version change do not request rotation.
The integration ID and installation should remain unchanged, though previously
issued installation tokens may remain valid until expiry. App ID replacement
requires a new browser installation handoff.

To test import of an existing integration, leave the credentials and version
unset and set the opt-in flag and App ID above. Set
`TF_VAR_github_app_enabled=false` if the integration is disabled. If the object
already belongs to a Terraform address, back up state and perform the
state-only handoff in the resource guide before importing. Then import its
Ona UUID:

```shell
unset TF_VAR_github_app_credentials TF_VAR_github_app_credentials_version
terraform -chdir=dev/local-devloop import \
  "${github_app_target}" '<ona-integration-uuid>'
terraform -chdir=dev/local-devloop plan \
  -target="${github_app_target}" -input=false
```

Import and Query must not require credentials, change the rotation marker, or
rotate secrets.

Delete only the dev-loop Ona integration with:

```shell
terraform -chdir=dev/local-devloop destroy \
  -target="${github_app_target}" -input=false
unset TF_VAR_enable_github_app_integration TF_VAR_github_app_id TF_VAR_github_app_enabled \
  TF_VAR_github_app_credentials TF_VAR_github_app_credentials_version
unset github_app_target
```

This leaves the GitHub App and its GitHub installation intact. Removing those
is a separate GitHub administrator action.

## Test service-account Git authentication

Service-account Git authentication is opt-in because it requires a real SCM
personal access token. Use Terraform 1.11 or later. The Ona API token must be
able to manage the runner, service account, and SCM integration used by the dev
loop. The GitHub PAT must have the repository access that the service account
will use.

Read both credentials without placing them in shell history, and configure the
local provider override created above:

```shell
read -rsp 'Ona API token: ' ONA_TOKEN && echo
export ONA_TOKEN

read -rsp 'GitHub personal access token: ' TF_VAR_git_personal_access_token && echo
export TF_VAR_git_personal_access_token

export TF_CLI_CONFIG_FILE="${PWD}/terraformrc"
export GIT_AUTH_TARGET='ona_git_authentication.devloop[0]'
export GIT_AUTH_VERSION='v1'
```

Apply only the Git authentication and its runner, service-account, and SCM
integration dependencies:

```shell
terraform -chdir=dev/local-devloop apply \
  -target="${GIT_AUTH_TARGET}" \
  -var='enable_git_authentication=true' \
  -var="git_personal_access_token_version=${GIT_AUTH_VERSION}" \
  -auto-approve -input=false
```

Record the created authentication ID, inspect its state, and run the same plan
again. The state must not contain a `personal_access_token` value, and the plan
should report no changes:

```shell
GIT_AUTH_ID="$(terraform -chdir=dev/local-devloop output -raw managed_git_authentication_id)"
terraform -chdir=dev/local-devloop state show "${GIT_AUTH_TARGET}"

terraform -chdir=dev/local-devloop plan \
  -target="${GIT_AUTH_TARGET}" \
  -var='enable_git_authentication=true' \
  -var="git_personal_access_token_version=${GIT_AUTH_VERSION}" \
  -input=false
```

To test in-place PAT rotation, read the replacement PAT, change the non-secret
version marker, and apply again. The final command must succeed, proving that
the authentication ID did not change:

```shell
read -rsp 'Replacement GitHub personal access token: ' TF_VAR_git_personal_access_token && echo
export TF_VAR_git_personal_access_token
export GIT_AUTH_VERSION='v2'

terraform -chdir=dev/local-devloop apply \
  -target="${GIT_AUTH_TARGET}" \
  -var='enable_git_authentication=true' \
  -var="git_personal_access_token_version=${GIT_AUTH_VERSION}" \
  -auto-approve -input=false

test "$(terraform -chdir=dev/local-devloop output -raw managed_git_authentication_id)" = "${GIT_AUTH_ID}"
```

The PAT is an ephemeral variable passed to a write-only resource argument, so
Terraform does not store it in plan or state. To remove only the Git
authentication association while leaving shared dev-loop dependencies intact,
keep the same variable values and target its resource instance:

```shell
terraform -chdir=dev/local-devloop destroy \
  -target="${GIT_AUTH_TARGET}" \
  -var='enable_git_authentication=true' \
  -var="git_personal_access_token_version=${GIT_AUTH_VERSION}" \
  -auto-approve -input=false

unset ONA_TOKEN TF_VAR_git_personal_access_token GIT_AUTH_TARGET GIT_AUTH_VERSION
```

Use the full cleanup command at the end of this README only when every resource
in the dev-loop state should be destroyed.

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

Automation sharing is opt-in because custom-group sharing requires an
Enterprise organization and an existing Automation. Grant the devloop group
permission to run an Automation with:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop apply \
  -var='automation_sharing_automation_id=<automation-id>' \
  -auto-approve -input=false
```

The assignment grants `executor` access to `ona_group.devloop`. Destroying the
assignment removes only the group access and does not delete the referenced
Automation.

The devloop always assigns its group a direct role on its project. The explicit
dependency on the organization role assignments gives the group runner access
before Ona validates the project assignment. Because this group also has the
organization-wide `projects_admin` role, its effective project access remains
admin even when the direct assignment uses `user` or `editor`; this workflow
exercises the direct assignment lifecycle.

To verify legacy import, remove only the assignment from local state and import
the still-existing Ona assignment by project and group ID:

```shell
project_id="$(TF_CLI_CONFIG_FILE="${PWD}/terraformrc" terraform -chdir=dev/local-devloop output -raw managed_project_id)"
group_id="$(TF_CLI_CONFIG_FILE="${PWD}/terraformrc" terraform -chdir=dev/local-devloop output -raw managed_group_id)"
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" terraform -chdir=dev/local-devloop state rm ona_project_role_assignment.devloop
ONA_TOKEN=... TF_CLI_CONFIG_FILE="${PWD}/terraformrc" terraform -chdir=dev/local-devloop import ona_project_role_assignment.devloop "${project_id}/${group_id}"
```

A subsequent plan should be empty. Exercise replacement by changing the direct
role:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop apply \
  -var='project_sharing_role=user' \
  -auto-approve -input=false
```

Use the same `project_sharing_role` value in later plans, or set it back to
`editor` to exercise a second replacement.

Runner sharing is opt-in because custom-group sharing requires an Enterprise
organization. Enable the lifecycle test with:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop apply \
  -var='runner_sharing_role=user' \
  -auto-approve -input=false
```

The dev loop creates a separate empty group so its direct runner assignment is
not hidden by `ona_group.devloop` having organization-wide `runners_admin`
access. Change the role to `admin` to exercise replacement. Use the same role
when destroying the dev loop so Terraform removes the assignment and its empty
group.

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

Clean up the resources afterward with:

```shell
ONA_TOKEN=... \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop destroy -auto-approve -input=false
rm -f /tmp/ona-webhook-secret.txt
```
