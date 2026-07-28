#!/usr/bin/env bash
# Inputs: ONA_TOKEN, ONA_HOST, TMPDIR, and TERRAFORM from the environment;
# optional output argument names a destination file for generated Terraform.
# Example: ./query.sh generated-environment-classes.tf
# Output: runs Terraform Query for query.hcl and prints the generated file path.
set -euo pipefail
# BASH_SOURCE points at this script even when it is invoked from elsewhere.
script_parent=$(dirname "${BASH_SOURCE[0]}")
script_dir=$(cd "$script_parent" >/dev/null && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
terraform_bin="${TERRAFORM:-terraform}"
output="${1:-}"
if [[ -n "$output" && -e "$output" ]]; then
  echo "$output already exists" >&2
  exit 1
fi
workdir="$(mktemp -d "${TMPDIR:-/tmp}/ona-environment-class-query.XXXXXX")"
cleanup_dir="$workdir"
if [[ -z "$output" ]]; then
  output="$workdir/generated.tf"
  cleanup_dir=""
fi
trap '[[ -z "$cleanup_dir" ]] || rm -rf "$cleanup_dir"' EXIT
bin_dir="$workdir/bin"
mkdir -p "$bin_dir"
go -C "$repo_root" build -trimpath -ldflags "-s -w -X main.version=dev" -o "$bin_dir/terraform-provider-ona" .
cat >"$workdir/terraformrc" <<EOF
provider_installation {
  dev_overrides { "registry.terraform.io/gitpod-io/ona" = "$bin_dir" }
  direct {}
}
EOF
cp "$script_dir/query.hcl" "$workdir/query.tfquery.hcl"
cat >"$workdir/provider.tf" <<'EOF'
terraform {
  required_version = ">= 1.14.0"
  required_providers { ona = { source = "gitpod-io/ona" } }
}
provider "ona" {}
EOF
TF_CLI_CONFIG_FILE="$workdir/terraformrc" "$terraform_bin" -chdir="$workdir" query -generate-config-out=generated.tf
[[ "$output" == "$workdir/generated.tf" ]] || cp "$workdir/generated.tf" "$output"
echo "Generated config: $output"
