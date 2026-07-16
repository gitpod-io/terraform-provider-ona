# Terraform Provider for Ona

The [Ona Provider](https://registry.terraform.io/providers/gitpod-io/ona/latest/docs)
enables [Terraform](https://developer.hashicorp.com/terraform) to manage Ona
projects, runners, identity and access settings, organization settings,
security controls, secrets, workflows, webhooks, and integrations.

- [Provider documentation](docs/)
- [Examples](examples/)
- [Release process](docs/release.md)
- [Changelog](CHANGELOG.md)
- [Support](https://ona.com/support)
- [Security reporting](https://github.com/gitpod-io/terraform-provider-ona/security/policy)

This provider is built with the
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

## Supported Features

The provider currently includes these Terraform types.

Managed resources:

- Projects and runner infrastructure: `ona_project`, `ona_runner`,
  `ona_environment_class`, `ona_warm_pool`, `ona_runner_policy`.
- Runner integrations: `ona_scm_integration`, `ona_runner_llm_integration`.
- Organization configuration: `ona_announcement_banner`, `ona_custom_domain`,
  `ona_organization_policies`, `ona_terms_of_service`.
- Identity and access: `ona_group`, `ona_group_membership`,
  `ona_organization_role_assignment`, `ona_service_account`,
  `ona_oidc_config`, `ona_scim_configuration`, `ona_sso_configuration`.
- Security and secrets: `ona_security_policy`, `ona_secret`.
- Automations and integrations: `ona_integration`, `ona_webhook`,
  `ona_automation`.

Data sources:

- `ona_integration_definitions`
- `ona_runner` and `ona_runners`
- `ona_security_policies`
- `ona_warm_pool` and `ona_warm_pools`
- `ona_workflows`

Ephemeral resources:

- `ona_runner_token`
- `ona_service_account_token`
- `ona_webhook_secret`

Terraform Query list resources:

- `ona_runner`

## Requirements

- [Terraform CLI](https://developer.hashicorp.com/terraform/downloads) >= 1.14
- [Go](https://go.dev/doc/install) >= 1.25.12 for provider development.

## Building the Provider

```shell
make build
```

## Developing the Provider

Download dependencies:

```shell
make install-dependencies
```

Run unit tests:

```shell
make test-unit
```

Run the full local test suite:

```shell
make test
```

Run acceptance tests:

```shell
make test-acc
```

Generate documentation:

```shell
make generate
```

## Service Account Credentials

Use a human or admin personal access token for the first Terraform apply that
creates service accounts and issues the initial service-account token. Store the
service-account token in an external secret manager, then use that token as
`ONA_TOKEN` for subsequent Terraform runs.

The backend rejects service-account-token-to-service-account-token management:
service-account tokens cannot create or rotate other service-account tokens. Run
token bootstrap or rotation with a human/admin token.

For organization-level settings, a service account needs organization-admin
authorization through group membership. Terraform resources for group
membership and organization role assignment are tracked separately from the
service-account resource. Create a custom group, add the service account with
`ona_group_membership`, and grant the group a supported organization role with
`ona_organization_role_assignment`.

## Releasing

Beta releases are prerelease builds for validation and feedback. They are not
stable releases and do not provide compatibility or availability guarantees.
See [docs/release.md](docs/release.md) for the publish procedure.

## Query Existing Resources

Terraform Query can discover existing Ona resources through provider list
resources and generate starter Terraform configuration. This workflow requires
Terraform CLI >= 1.14.

See [examples/query.md](examples/query.md) for the runner query workflow.

## Import Existing Resources

The Terraform-native import workflow is:

1. discover existing Ona resources through the provider,
2. create Terraform import blocks,
3. run Terraform config generation,
4. apply the imports, and
5. verify that the resulting plan is a no-op.

Every managed resource listed above implements Terraform import support. See
the generated resource docs under [docs/resources](docs/resources/) for
resource-specific import IDs and examples.

The import helper in [scripts](scripts/) still generates import blocks only for
`ona_project`, `ona_runner`, and `ona_environment_class` resources. It
discovers additional Ona objects for inventory and reference rewriting, but
those resource families need helper selection support before the script can
generate import blocks for them.

See [examples/import.md](examples/import.md) for the full workflow and flags.
