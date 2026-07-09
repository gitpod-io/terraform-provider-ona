#!/usr/bin/env bash

set -euo pipefail

REGISTRY_PROVIDER="${REGISTRY_PROVIDER:-gitpod-io/ona}"

die() {
	echo "::error::$*" >&2
	exit 1
}

need_command() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

main() {
	need_command curl

	registry_url="https://registry.terraform.io/v1/providers/${REGISTRY_PROVIDER}"
	if ! curl --fail --silent --show-error --location "$registry_url" >/dev/null; then
		die "Terraform Registry provider ${REGISTRY_PROVIDER} is not reachable. Register it and add the signing public key before registry ingestion."
	fi

	echo "Terraform Registry provider is reachable: ${REGISTRY_PROVIDER}"
}

main "$@"
