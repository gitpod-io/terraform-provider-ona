#!/usr/bin/env bash

set -euo pipefail

RELEASE_REPOSITORY="${RELEASE_REPOSITORY:-gitpod-io/terraform-provider-ona}"
VERSION="${VERSION:-}"

die() {
	echo "::error::$*" >&2
	exit 1
}

need_command() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

normalize_version() {
	local version="$1"
	version="${version//[[:space:]]/}"
	[[ -n "$version" ]] || die "VERSION is required, for example v0.1.0-beta.1"
	[[ "$version" == v* ]] || version="v${version}"
	if ! [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
		die "VERSION must be a Terraform-compatible semver such as v0.1.0 or v0.1.0-beta.1, got: $version"
	fi
	printf '%s' "$version"
}

require_publish_inputs() {
	local -a missing=()

	[[ -n "$VERSION" ]] || missing+=("VERSION")
	[[ -n "${GPG_FINGERPRINT:-}" ]] || missing+=("GPG_FINGERPRINT")
	[[ -n "${GPG_PRIVATE_KEY:-}" ]] || missing+=("GPG_PRIVATE_KEY")
	if [[ "${#missing[@]}" -gt 0 ]]; then
		die "missing required publish inputs or secrets: ${missing[*]}"
	fi
}

main() {
	local version

	need_command gh

	require_publish_inputs
	version="$(normalize_version "$VERSION")"

	gh repo view "$RELEASE_REPOSITORY" --json nameWithOwner --jq .nameWithOwner >/dev/null

	if gh release view "$version" --repo "$RELEASE_REPOSITORY" >/dev/null 2>&1; then
		die "release ${version} already exists in ${RELEASE_REPOSITORY}"
	fi

	if gh api "repos/${RELEASE_REPOSITORY}/git/ref/tags/${version}" >/dev/null 2>&1; then
		die "tag ${version} already exists in ${RELEASE_REPOSITORY}"
	fi

	echo "Publish preflight passed for ${version}:"
	echo "  release repository: ${RELEASE_REPOSITORY}"
}

main "$@"
