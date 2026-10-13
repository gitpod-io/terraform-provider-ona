## 0.5.0-beta (Unreleased)

BREAKING CHANGES:

- Ona now manages authentication for built-in integrations. Remove custom
  authentication and credential overrides before upgrading.
  [Migration details](docs/resources/integration.md).

FEATURES:

- Manage [organization-owned GitHub App integrations](docs/resources/github_app_integration.md)
  through Terraform.

IMPROVEMENTS:

- Runner details now link to [AWS and GCP Terraform deployment modules](docs/resources/runner.md).
- Manage larger [Enterprise Automations](docs/resources/automation.md), with
  up to 1,000 projects and 1,000 total actions.

## 0.4.0-beta (Unreleased)

BREAKING CHANGES:

- Removed the `ona_runner_policy` resource and its Terraform Query list
  resource. This removes Terraform management of the policy-based **Shared
  with everyone in org** setting. It does not change group-based sharing.

  Remove every `ona_runner_policy` address from state before upgrading. Use
  `0.3.0-beta.49`, a provider version that supports the resource, for the
  cleanup apply.

  To delete the remote org-wide share, pin `0.3.0-beta.49`, remove the resource
  block, and apply:

  ```shell
  terraform init -upgrade
  terraform apply
  ```

  The delete can fail when a shared project depends on the runner. Unshare the
  affected project or move it to another runner, then apply again.

  To leave the remote org-wide share unchanged, replace each resource block
  with a `removed` block and apply with `0.3.0-beta.49`:

  ```hcl
  removed {
    from = ona_runner_policy.org_members

    lifecycle {
      destroy = false
    }
  }
  ```

  You can instead detach an address directly:

  ```shell
  terraform state rm 'ona_runner_policy.org_members'
  ```

  Confirm that `terraform state list` contains no `ona_runner_policy`
  addresses. Remove any temporary `removed` blocks, then update the provider
  constraint and run `terraform init -upgrade`. If you upgraded first and
  Terraform reports an unsupported resource type, pin `0.3.0-beta.49` again,
  run `terraform init -upgrade`, and complete one of the cleanup paths above.

## 0.2.0 (Unreleased)

FEATURES:
