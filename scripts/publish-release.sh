#!/usr/bin/env bash

set -euo pipefail

PROVIDER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_REPOSITORY="${RELEASE_REPOSITORY:-gitpod-io/terraform-provider-ona}"
VERSION="${VERSION:-}"
DRY_RUN="${DRY_RUN:-0}"
TEMP_GNUPGHOME=""
TEMP_DOWNLOAD_DIRS=()

die() {
	echo "::error::$*" >&2
	exit 1
}

need_command() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

cleanup() {
	if [[ -n "$TEMP_GNUPGHOME" ]]; then
		rm -rf "$TEMP_GNUPGHOME"
	fi
	if [[ "${#TEMP_DOWNLOAD_DIRS[@]}" -gt 0 ]]; then
		rm -rf "${TEMP_DOWNLOAD_DIRS[@]}"
	fi
}

stage() {
	printf '==> %s\n' "$*" >&2
}

normalize_repo() {
	local repo="$1"
	repo="${repo#https://github.com/}"
	repo="${repo#git@github.com:}"
	repo="${repo%.git}"
	printf '%s' "$repo"
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

require_github_actions_main() {
	if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
		die "publishing is only supported from the manual GitHub Actions release workflow"
	fi
	if [[ "${GITHUB_REF:-}" != "refs/heads/main" ]]; then
		die "publishing must run from main after the release-prep PR merges, got ${GITHUB_REF:-<unset>}"
	fi
}

validate_release_metadata() {
	local version="$1"

	VERSION_FILE="${PROVIDER_DIR}/VERSION" \
	CHANGELOG_FILE="${PROVIDER_DIR}/CHANGELOG.md" \
		"${PROVIDER_DIR}/scripts/validate-release-version.sh" --expect-tag "$version" >/dev/null
}

import_gpg_key() {
	[[ -n "${GPG_PRIVATE_KEY:-}" ]] || die "GPG_PRIVATE_KEY is required"
	[[ -n "${GPG_FINGERPRINT:-}" ]] || die "GPG_FINGERPRINT is required"

	if [[ -z "${GNUPGHOME:-}" ]]; then
		TEMP_GNUPGHOME="$(mktemp -d)"
		export GNUPGHOME="$TEMP_GNUPGHOME"
	fi
	chmod 700 "$GNUPGHOME"
	printf '%s' "$GPG_PRIVATE_KEY" | gpg --batch --import
}

sign_checksums() {
	local checksum_file="$1"
	local signature_file="${checksum_file}.sig"
	local -a gpg_args=(
		--batch
		--yes
		--local-user "$GPG_FINGERPRINT"
		--output "$signature_file"
		--detach-sign "$checksum_file"
	)

	if [[ -n "${GPG_PASSPHRASE:-}" ]]; then
		gpg_args+=(--pinentry-mode loopback --passphrase-fd 0)
		gpg "${gpg_args[@]}" <<<"$GPG_PASSPHRASE"
	else
		gpg "${gpg_args[@]}"
	fi

	gpg --batch --verify "$signature_file" "$checksum_file"
}

build_release_artifacts() {
	local version="$1"
	local dist="${PROVIDER_DIR}/dist"

	DIST_DIR="$dist" VERSION="$version" "${PROVIDER_DIR}/scripts/build-release-artifacts.sh"
}

verify_local_artifacts() {
	local version="$1"
	local dist="${PROVIDER_DIR}/dist"

	"${PROVIDER_DIR}/scripts/verify-release-artifacts.sh" "$dist" "$version" --require-signature
}

write_release_notes() {
	local version="$1"
	local notes="${PROVIDER_DIR}/dist/release-notes.md"

	cat >"$notes" <<EOF
Terraform Provider for Ona ${version}
EOF

	printf '%s' "$notes"
}

github_preflight() {
	local repo="$1"
	local version="$2"

	gh repo view "$repo" --json nameWithOwner --jq .nameWithOwner >/dev/null

	if gh release view "$version" --repo "$repo" >/dev/null 2>&1; then
		die "release ${version} already exists in ${repo}"
	fi

	if gh api "repos/${repo}/git/ref/tags/${version}" >/dev/null 2>&1; then
		die "tag ${version} already exists in ${repo}"
	fi

	gh repo view "$repo" --json defaultBranchRef --jq .defaultBranchRef.name
}

create_github_release() {
	local repo="$1"
	local version="$2"
	local default_branch="$3"
	local notes="$4"
	local version_no_v="${version#v}"
	local dist="${PROVIDER_DIR}/dist"

	local assets=(
		"${dist}/terraform-provider-ona_${version_no_v}_linux_amd64.zip"
		"${dist}/terraform-provider-ona_${version_no_v}_linux_arm64.zip"
		"${dist}/terraform-provider-ona_${version_no_v}_manifest.json"
		"${dist}/terraform-provider-ona_${version_no_v}_SHA256SUMS"
		"${dist}/terraform-provider-ona_${version_no_v}_SHA256SUMS.sig"
	)

	local release_args=(
		"$version"
		"${assets[@]}"
		--repo "$repo"
		--target "$default_branch"
		--title "$version"
		--notes-file "$notes"
	)

	if [[ "$version" == *-* ]]; then
		release_args+=(--prerelease)
	fi

	if [[ "$DRY_RUN" == "1" ]]; then
		printf 'Dry run: would create GitHub release %s in %s targeting %s with %d assets\n' \
			"$version" "$repo" "$default_branch" "${#assets[@]}"
		return
	fi

	gh release create "${release_args[@]}"
}

verify_published_release() {
	local repo="$1"
	local version="$2"
	local version_no_v="${version#v}"
	local download_dir
	download_dir="$(mktemp -d)"
	TEMP_DOWNLOAD_DIRS+=("$download_dir")

	gh release view "$version" \
		--repo "$repo" \
		--json tagName,isDraft,isPrerelease,url,assets \
		--jq '{tagName,isDraft,isPrerelease,url,assets:[.assets[].name]}'

	gh release download "$version" --repo "$repo" --dir "$download_dir"
	"${PROVIDER_DIR}/scripts/verify-release-artifacts.sh" "$download_dir" "$version_no_v" --require-signature
}

main() {
	local version
	local repo
	local notes
	local default_branch

	trap cleanup EXIT

	stage "Validate publish inputs"
	require_github_actions_main
	version="$(normalize_version "$VERSION")"
	repo="$(normalize_repo "$RELEASE_REPOSITORY")"
	validate_release_metadata "$version"

	stage "Check required commands"
	need_command git
	need_command go
	need_command gh
	need_command gpg
	need_command jq
	need_command sha256sum
	need_command zip
	need_command zipinfo

	stage "Import GPG signing key"
	import_gpg_key
	stage "Build and package release artifacts"
	build_release_artifacts "$version"

	local checksum_file="${PROVIDER_DIR}/dist/terraform-provider-ona_${version#v}_SHA256SUMS"
	stage "Sign checksum file"
	sign_checksums "$checksum_file"
	stage "Verify local release artifacts"
	verify_local_artifacts "$version"
	stage "Write release notes"
	notes="$(write_release_notes "$version")"

	stage "Check public GitHub release target"
	default_branch="$(github_preflight "$repo" "$version")"
	stage "Create GitHub release"
	create_github_release "$repo" "$version" "$default_branch" "$notes"

	if [[ "$DRY_RUN" != "1" ]]; then
		stage "Verify published release assets"
		verify_published_release "$repo" "$version"
	fi
}

main "$@"
