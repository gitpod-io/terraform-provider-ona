#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"; group_id="${GROUP_ID:?Set GROUP_ID}"; output="${1:-generated.tf}"
if [[ -e "$output" ]]; then echo "$output already exists" >&2; exit 1; fi
workdir="$(mktemp -d)"; trap 'rm -rf "$workdir"' EXIT
cp query.hcl "$workdir/query.tfquery.hcl"
cat >"$workdir/provider.tf" <<'EOF'
terraform { required_providers { ona = { source = "gitpod-io/ona" } } }
provider "ona" {}
EOF
terraform -chdir="$workdir" init
terraform -chdir="$workdir" query -var="group_id=$group_id" -generate-config-out=generated.tf
cp "$workdir/generated.tf" "$output"
