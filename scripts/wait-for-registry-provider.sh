#!/usr/bin/env bash

set -euo pipefail

SOURCE="${SOURCE:-${REGISTRY_PROVIDER_SOURCE:-gitpod-io/ona}}"
VERSION="${VERSION:-}"
TIMEOUT_MINUTES="${TIMEOUT_MINUTES:-60}"

die() {
	echo "::error::$*" >&2
	exit 1
}

need_command() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

write_output() {
	local name="$1"
	local value="$2"

	if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
		echo "${name}=${value}" >>"$GITHUB_OUTPUT"
	fi
}

main() {
	local source="$SOURCE"
	local version="${VERSION#v}"
	local timeout_minutes="$TIMEOUT_MINUTES"
	local first second third rest
	local namespace type endpoint deadline

	need_command curl
	need_command jq

	[[ -n "$source" ]] || die "SOURCE is required, for example gitpod-io/ona"
	[[ -n "$version" ]] || die "VERSION is required, for example 0.1.0-beta.1"
	[[ "$timeout_minutes" =~ ^[0-9]+$ ]] || die "TIMEOUT_MINUTES must be a non-negative integer"

	IFS=/ read -r first second third rest <<<"$source"

	if [[ -n "${rest:-}" || -z "${second:-}" ]]; then
		die "Expected source to be namespace/type or registry.terraform.io/namespace/type."
	fi

	if [[ -z "${third:-}" ]]; then
		namespace="$first"
		type="$second"
	elif [[ "$first" == "registry.terraform.io" ]]; then
		namespace="$second"
		type="$third"
	else
		die "Only registry.terraform.io sources are supported by this check."
	fi

	endpoint="https://registry.terraform.io/v1/providers/${namespace}/${type}/versions"
	deadline=$((SECONDS + timeout_minutes * 60))

	while true; do
		if curl -fsSL "$endpoint" | jq -e --arg version "$version" '.versions[]? | select(.version == $version)' >/dev/null; then
			echo "Terraform Registry has ${source} ${version}."
			write_output source "$source"
			write_output version "$version"
			write_output timeout_minutes "$timeout_minutes"
			exit 0
		fi

		if ((SECONDS >= deadline)); then
			die "Timed out waiting for ${source} ${version} in the Terraform Registry."
		fi

		echo "Waiting for ${source} ${version}..."
		sleep 60
	done
}

main "$@"
