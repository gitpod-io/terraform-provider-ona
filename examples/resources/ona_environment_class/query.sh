#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

runner_id="${RUNNER_ID:?Set RUNNER_ID to the runner to query}"
output="${1:-generated.tf}"
if [[ -e "$output" ]]; then
  echo "$output already exists" >&2
  exit 1
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

cp query.hcl "$workdir/query.tfquery.hcl"
cat >"$workdir/provider.tf" <<'EOF'
terraform {
  required_providers {
    ona = {
      source = "gitpod-io/ona"
    }
  }
}

provider "ona" {}
EOF

terraform -chdir="$workdir" init
terraform -chdir="$workdir" query -var="runner_id=$runner_id" -generate-config-out=generated.tf
cp "$workdir/generated.tf" "$output"
