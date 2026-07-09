# Local Dev Loop Module

This module exercises all Terraform provider resources and both runner data sources:

- `ona_runner.devloop`
- `ona_environment_class.devloop`
- `ona_project.devloop`
- `ona_scm_integration.github_oauth`
- `ona_scm_integration.gitlab_pat`
- `ona_scm_integration.azuredevops_entra`
- `ona_scm_integration.azuredevops_server`
- `ephemeral.ona_runner_token.devloop`
- `data.ona_runners.all`
- `data.ona_runner.devloop`

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

The apply output includes `cloudformation_template_url` for AWS EC2 runners.
Runner registration tokens are consumed through `ephemeral.ona_runner_token`
during apply, so they are not written as normal Terraform outputs or stored in
state.

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
```
