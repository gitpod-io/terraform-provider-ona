# Terraform Provider for Ona

## Overview

The Ona Terraform provider manages Ona organization configuration as code. It
is intended for platform, identity, and security teams that administer Ona
projects, runners, access controls, policies, secrets, and automations.

The provider is currently beta software. The current published release is
`0.3.0-beta.5`.

- [Terraform Registry provider documentation](https://registry.terraform.io/providers/gitpod-io/ona/latest/docs)
- [Ona documentation](https://ona.com/docs/ona/getting-started)
- [Source code](./)
- [Changelog](CHANGELOG.md)

## Requirements

Use Terraform CLI `1.14` or later. The published beta supports:

- Linux `amd64`
- Linux `arm64`

macOS and Windows provider packages are not currently available.

## Quick start

Pin the published beta version intentionally:

```hcl
terraform {
  required_version = ">= 1.14"

  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "= 0.3.0-beta.5"
    }
  }
}

provider "ona" {}
```

Create a read-and-write [personal access token](https://ona.com/docs/ona/integrations/personal-access-token),
keep it outside the Terraform configuration, and run:

```shell
export ONA_TOKEN="<ona-personal-access-token>"
terraform init
terraform validate
terraform plan
```

Most users should omit `ONA_HOST`. Set it only when your organization uses a
non-default Ona application host:

```shell
export ONA_HOST="https://<ona-hostname>"
```

## Supported capabilities

The provider covers these durable capability categories:

- **Projects:** project configuration and project runtime capacity.
- **Runners:** runners, environment classes, warm pools, policies, and runner
  integrations.
- **Identity and access:** groups, service accounts, role assignments, SSO,
  SCIM, and OIDC configuration.
- **Organization settings:** organization policies, announcements, custom
  domains, and terms of service.
- **Security and secrets:** security policies and scoped secrets.
- **Integrations and automation:** integrations, webhooks, and automations.

The [Terraform Registry navigation](https://registry.terraform.io/providers/gitpod-io/ona/latest/docs)
is the authoritative inventory for each published version's managed resources,
data sources, ephemeral resources, and list resources. The generated source for
that reference is also checked into [docs/](docs/).

## Authentication and permissions

Use a personal access token (PAT) for Terraform write operations. PATs inherit
the user's permissions and can be created with read-only or read-and-write
access; a read-only PAT is suitable only for operations that do not modify Ona.
Choose the shortest practical expiry and revoke tokens that are no longer used.
See [Personal access tokens](https://ona.com/docs/ona/integrations/personal-access-token).

Use service-account tokens only within their documented scope. Ona currently
documents them for starting automations and performing API read operations, and
directs users to contact Ona for additional use cases. Do not assume a
service-account token can create, update, or delete provider-managed objects.
See [Service accounts](https://ona.com/docs/ona/organizations/service-accounts).

Only organization admins can create service-account tokens. Bootstrap and
rotate them in a run authenticated with an authorized human or administrator
PAT, then store the returned token in an external secret manager. Every token
must also have permission to access the organization and objects used by the
Terraform configuration.

## State and secret safety

Terraform's `Sensitive` marking redacts values in normal CLI output but does
not keep ordinary attributes out of state. Treat state and saved plans as
sensitive, and use the provider's write-only arguments and ephemeral resources
when handling secret material. See the Registry guide to
[state, secrets, and safe deletion](https://registry.terraform.io/providers/gitpod-io/ona/latest/docs/guides/state-secrets-and-safe-deletion).

Removing a managed resource from configuration normally plans a destroy and
may delete or disable the remote Ona object. By contrast, removing its address
with `terraform state rm <address>` changes only Terraform state and leaves the
remote object in place. Review every destroy plan before applying it.

## Documentation and support

- [Terraform Registry provider documentation](https://registry.terraform.io/providers/gitpod-io/ona/latest/docs)
- [Generated provider reference in this repository](docs/)
- [Public Ona documentation](https://ona.com/docs/ona/getting-started)
- [Changelog](CHANGELOG.md)
- [Security reporting](SECURITY.md)
- [Ona support](https://ona.com/support)

## Contributing

Use [the contributing guide](contributing.md) for repository setup and policy.
External pull requests are not currently accepted, but the repository can be
cloned for local development. Contributors need Go `1.25.12`, Terraform `1.14`
or later, GNU Make, `golangci-lint`, and ShellCheck.

Use the Makefile targets as the command source of truth:

```shell
make install-dependencies
make fmt
make generate
make lint
make test-unit
make build
```

`make test` runs the unit and acceptance test suites. Run it, `make test-acc`,
or the [local Terraform development loop](dev/local-devloop/README.md) only when
you are authorized to perform credentialed operations and have the required Ona
credentials.

Never commit tokens, private keys, Terraform state or saved plans, local
provider override files, or release signing material.

## Releasing

See the [release process](docs/release.md) for release preparation, publishing,
and verification.
