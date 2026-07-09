#!/usr/bin/env bash

set -euo pipefail

PROVIDER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-${1:-}}"
DIST_DIR="${DIST_DIR:-${PROVIDER_DIR}/dist/release-snapshot}"

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

build_binary() {
	local goos="$1"
	local goarch="$2"
	local output="$3"
	local version="$4"

	mkdir -p "$(dirname "$output")"
	(
		cd "$PROVIDER_DIR"
		GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
			go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "$output" .
	)
}

main() {
	local version
	local version_no_v

	need_command go
	need_command jq
	need_command sha256sum
	need_command zip
	need_command zipinfo

	version="$(normalize_version "$VERSION")"
	version_no_v="${version#v}"

	rm -rf "$DIST_DIR"
	mkdir -p "$DIST_DIR"

	build_binary linux amd64 "${DIST_DIR}/build/linux_amd64/terraform-provider-ona" "$version"
	build_binary linux arm64 "${DIST_DIR}/build/linux_arm64/terraform-provider-ona" "$version"
	cp "$PROVIDER_DIR/terraform-registry-manifest.json" "$DIST_DIR/terraform-registry-manifest.json"

	(
		cd "$DIST_DIR"
		"$PROVIDER_DIR/scripts/package-release-artifacts.sh" "$version_no_v"
	)
	"$PROVIDER_DIR/scripts/verify-release-artifacts.sh" "$DIST_DIR" "$version_no_v"
}

main "$@"
