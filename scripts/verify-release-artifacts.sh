#!/usr/bin/env bash

set -euo pipefail

artifact_dir="${1:-}"
version="${2:-}"
signature_mode="${3:-}"
provider="terraform-provider-ona"

die() {
	echo "::error::$*" >&2
	exit 1
}

need_command() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

print_lines() {
	local title="$1"
	shift

	echo "$title"
	printf '  %s\n' "$@"
}

sorted_lines() {
	printf '%s\n' "$@" | LC_ALL=C sort
}

require_exact_files() {
	local -n expected_ref=$1
	local -a actual=()
	local expected_list
	local actual_list
	local path

	shopt -s nullglob
	for path in "$artifact_dir"/*; do
		[[ -f "$path" ]] || continue
		actual+=("$(basename "$path")")
	done
	shopt -u nullglob

	expected_list="$(sorted_lines "${expected_ref[@]}")"
	actual_list="$(sorted_lines "${actual[@]}")"

	if [[ "$expected_list" != "$actual_list" ]]; then
		print_lines "Expected release artifacts:" "${expected_ref[@]}" >&2
		print_lines "Found release artifacts:" "${actual[@]}" >&2
		die "release artifact inventory mismatch in $artifact_dir"
	fi
}

require_checksum_entries() {
	local checksum_file="$1"
	shift
	local -a expected_entries=("$@")
	local -a actual_entries=()
	local expected_list
	local actual_list

	mapfile -t actual_entries < <(awk '{print $2}' "$checksum_file" | sed 's/^\*//' | LC_ALL=C sort)
	expected_list="$(sorted_lines "${expected_entries[@]}")"
	actual_list="$(printf '%s\n' "${actual_entries[@]}")"

	if [[ "$expected_list" != "$actual_list" ]]; then
		print_lines "Expected checksum entries:" "${expected_entries[@]}" >&2
		print_lines "Found checksum entries:" "${actual_entries[@]}" >&2
		die "checksum file does not match expected release artifacts: $checksum_file"
	fi
}

require_zip_contents() {
	local zip_name="$1"
	local expected_binary="$2"
	local zip_file="${artifact_dir}/${zip_name}"
	local contents

	contents="$(zipinfo -1 "$zip_file" | LC_ALL=C sort)"
	if [[ "$contents" != "$expected_binary" ]]; then
		echo "Expected ${zip_name} to contain only: ${expected_binary}" >&2
		echo "Found:" >&2
		printf '%s\n' "$contents" >&2
		die "unexpected zip contents: $zip_file"
	fi
}

require_manifest() {
	local manifest_file="$1"

	if ! jq -e '.version == 1 and (.metadata.protocol_versions == ["6.0"])' "$manifest_file" >/dev/null; then
		die "manifest must declare version 1 and Terraform protocol version 6.0: $manifest_file"
	fi
}

if [[ -z "$artifact_dir" || -z "$version" ]]; then
	die "usage: $0 <artifact-dir> <version> [--require-signature]"
fi
if [[ ! -d "$artifact_dir" ]]; then
	die "artifact directory does not exist: $artifact_dir"
fi
if [[ "$signature_mode" != "" && "$signature_mode" != "--require-signature" ]]; then
	die "unknown argument: $signature_mode"
fi

need_command awk
need_command jq
need_command sed
need_command sha256sum
need_command sort
need_command zipinfo
if [[ "$signature_mode" == "--require-signature" ]]; then
	need_command gpg
fi

version="${version#v}"
[[ -n "$version" ]] || die "version is required"

linux_amd64_zip="${provider}_${version}_linux_amd64.zip"
linux_arm64_zip="${provider}_${version}_linux_arm64.zip"
manifest_name="${provider}_${version}_manifest.json"
checksum_name="${provider}_${version}_SHA256SUMS"
signature_name="${checksum_name}.sig"
checksum_file="${artifact_dir}/${checksum_name}"
manifest_file="${artifact_dir}/${manifest_name}"
binary_name="${provider}_v${version}"

unsigned_artifacts=(
	"$linux_amd64_zip"
	"$linux_arm64_zip"
	"$manifest_name"
)
expected_artifacts=(
	"${unsigned_artifacts[@]}"
	"$checksum_name"
)

if [[ "$signature_mode" == "--require-signature" ]]; then
	expected_artifacts+=("$signature_name")
fi

require_exact_files expected_artifacts
require_checksum_entries "$checksum_file" "${unsigned_artifacts[@]}"
require_zip_contents "$linux_amd64_zip" "$binary_name"
require_zip_contents "$linux_arm64_zip" "$binary_name"
require_manifest "$manifest_file"

(
	cd "$artifact_dir"
	sha256sum -c "$checksum_name"
	if [[ "$signature_mode" == "--require-signature" ]]; then
		gpg --batch --verify "$signature_name" "$checksum_name"
	fi
)

echo "Verified Terraform provider release artifacts for ${version}:"
print_lines "Artifacts:" "${expected_artifacts[@]}"
