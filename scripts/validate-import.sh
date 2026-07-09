#!/usr/bin/env bash
set -euo pipefail

provider_source="registry.terraform.io/gitpod-io/ona"
terraform_bin="${TERRAFORM:-terraform}"
host="${ONA_HOST:-https://app.gitpod.io}"
token="${ONA_TOKEN:-}"
runner_id="${ONA_RUNNER_ID:-}"

if [ -z "$token" ]; then
  echo "missing ONA_TOKEN: set it to an Ona API token for the target organization" >&2
  exit 1
fi

if [ -z "$runner_id" ]; then
  echo "missing ONA_RUNNER_ID: set it to the ID of an existing runner to import" >&2
  exit 1
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/terraform-provider-ona-import.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/bin" "$tmp/work"

echo "building local Terraform provider"
go build -trimpath -ldflags "-s -w -X main.version=dev" -o "$tmp/bin/terraform-provider-ona" .

cat > "$tmp/terraformrc" <<EOF
provider_installation {
  dev_overrides {
    "$provider_source" = "$tmp/bin"
  }
  direct {}
}
EOF

cat > "$tmp/work/main.tf" <<'EOF'
terraform {
  required_providers {
    ona = {
      source = "gitpod-io/ona"
    }
  }
}

provider "ona" {}

resource "ona_runner" "validation" {}
EOF

export TF_CLI_CONFIG_FILE="$tmp/terraformrc"
export TF_IN_AUTOMATION=1
export ONA_HOST="$host"
export ONA_TOKEN="$token"

echo "validating Terraform configuration"
"$terraform_bin" -chdir="$tmp/work" validate -no-color

echo "importing runner $runner_id from $host"
"$terraform_bin" -chdir="$tmp/work" import -input=false -no-color ona_runner.validation "$runner_id"

echo "checking imported Terraform state"
"$terraform_bin" -chdir="$tmp/work" show -json > "$tmp/state.json"
jq -e --arg runner_id "$runner_id" '
  .values.root_module.resources[]
  | select(.address == "ona_runner.validation")
  | .values.id == $runner_id
    and (.values.name | type == "string" and length > 0)
' "$tmp/state.json" >/dev/null
runner_name=$(jq -r '
  .values.root_module.resources[]
  | select(.address == "ona_runner.validation")
  | .values.name
' "$tmp/state.json")
echo "imported runner: $runner_id ($runner_name)"

echo "checking imported runner plan is a no-op"
set +e
plan_output=$("$terraform_bin" -chdir="$tmp/work" plan -input=false -detailed-exitcode -out="$tmp/validation.tfplan" -no-color 2>&1)
plan_status=$?
set -e

case "$plan_status" in
  0)
    echo "$plan_output"
    echo "success: imported runner is in sync with the backend"
    ;;
  2)
    echo "$plan_output" >&2
    echo "terraform plan proposed changes after import:" >&2
    "$terraform_bin" -chdir="$tmp/work" show -no-color "$tmp/validation.tfplan" >&2
    exit 1
    ;;
  *)
    echo "$plan_output" >&2
    exit "$plan_status"
    ;;
esac
