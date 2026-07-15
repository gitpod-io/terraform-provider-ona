#!/usr/bin/env bash

set -euo pipefail

VERSION_FILE="${VERSION_FILE:-version/VERSION}"
CHANGELOG_FILE="${CHANGELOG_FILE:-CHANGELOG.md}"
GITHUB_OUTPUT_MODE=0
CHECK_TAG_PRECEDENCE=1
EXPECTED_VERSION=""

die() {
	echo "::error::$*" >&2
	exit 1
}

usage() {
	cat >&2 <<'EOF'
usage: scripts/validate-release-version.sh [--github-output] [--no-tag-precedence] [--expect-version <version>] [--expect-tag <tag>]

Validates that version/VERSION contains bare SemVer, CHANGELOG.md starts with
the same version, and the version is greater than existing local v* Git tags.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--github-output)
			GITHUB_OUTPUT_MODE=1
			;;
		--no-tag-precedence)
			CHECK_TAG_PRECEDENCE=0
			;;
		--expect-version)
			[[ $# -ge 2 ]] || die "--expect-version requires a value"
			EXPECTED_VERSION="$2"
			shift
			;;
		--expect-tag)
			[[ $# -ge 2 ]] || die "--expect-tag requires a value"
			EXPECTED_VERSION="$2"
			shift
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			die "unknown argument: $1"
			;;
	esac
	shift
done

is_integer() {
	[[ "$1" =~ ^(0|[1-9][0-9]*)$ ]]
}

is_numeric_identifier() {
	[[ "$1" =~ ^[0-9]+$ ]]
}

validate_semver() {
	local version="$1"
	local core prerelease major minor patch identifier

	[[ "$version" != v* ]] || die "${VERSION_FILE} must contain bare SemVer without a leading v: ${version}"
	[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]] || \
		die "${VERSION_FILE} must contain SemVer such as 0.2.0 or 0.2.0-beta.1: ${version}"

	core="${version%%-*}"
	IFS=. read -r major minor patch <<<"$core"
	is_integer "$major" || die "major version must be a non-negative integer without leading zeros: ${version}"
	is_integer "$minor" || die "minor version must be a non-negative integer without leading zeros: ${version}"
	is_integer "$patch" || die "patch version must be a non-negative integer without leading zeros: ${version}"

	if [[ "$version" == *-* ]]; then
		prerelease="${version#*-}"
		IFS=. read -ra identifiers <<<"$prerelease"
		for identifier in "${identifiers[@]}"; do
			[[ -n "$identifier" ]] || die "prerelease identifiers cannot be empty: ${version}"
			[[ "$identifier" =~ ^[0-9A-Za-z-]+$ ]] || die "invalid prerelease identifier '${identifier}' in ${version}"
			if is_numeric_identifier "$identifier" && [[ "$identifier" =~ ^0[0-9]+$ ]]; then
				die "numeric prerelease identifiers cannot contain leading zeros: ${version}"
			fi
		done
	fi
}

semver_gt() {
	local a="$1"
	local b="$2"
	local a_core b_core a_pre b_pre
	local a_major a_minor a_patch b_major b_minor b_patch
	local i max_len a_id b_id

	a_core="${a%%-*}"
	b_core="${b%%-*}"
	a_pre=""
	b_pre=""
	[[ "$a" == *-* ]] && a_pre="${a#*-}"
	[[ "$b" == *-* ]] && b_pre="${b#*-}"

	IFS=. read -r a_major a_minor a_patch <<<"$a_core"
	IFS=. read -r b_major b_minor b_patch <<<"$b_core"

	if ((a_major != b_major)); then
		((a_major > b_major))
		return
	fi
	if ((a_minor != b_minor)); then
		((a_minor > b_minor))
		return
	fi
	if ((a_patch != b_patch)); then
		((a_patch > b_patch))
		return
	fi

	if [[ -z "$a_pre" && -n "$b_pre" ]]; then
		return 0
	fi
	if [[ -n "$a_pre" && -z "$b_pre" ]]; then
		return 1
	fi
	if [[ -z "$a_pre" && -z "$b_pre" ]]; then
		return 1
	fi

	IFS=. read -ra a_ids <<<"$a_pre"
	IFS=. read -ra b_ids <<<"$b_pre"
	max_len="${#a_ids[@]}"
	if ((${#b_ids[@]} > max_len)); then
		max_len="${#b_ids[@]}"
	fi

	for ((i = 0; i < max_len; i++)); do
		a_id="${a_ids[$i]:-}"
		b_id="${b_ids[$i]:-}"

		if [[ -z "$a_id" && -n "$b_id" ]]; then
			return 1
		fi
		if [[ -n "$a_id" && -z "$b_id" ]]; then
			return 0
		fi
		if [[ "$a_id" == "$b_id" ]]; then
			continue
		fi

		if is_numeric_identifier "$a_id" && is_numeric_identifier "$b_id"; then
			((a_id > b_id))
			return
		fi
		if is_numeric_identifier "$a_id"; then
			return 1
		fi
		if is_numeric_identifier "$b_id"; then
			return 0
		fi
		[[ "$a_id" > "$b_id" ]]
		return
	done

	return 1
}

read_release_version() {
	[[ -f "$VERSION_FILE" ]] || die "missing ${VERSION_FILE}"

	local version
	local -a lines=()
	mapfile -t lines <"$VERSION_FILE"

	[[ "${#lines[@]}" == "1" ]] || die "${VERSION_FILE} must contain exactly one line"
	version="${lines[0]//[[:space:]]/}"
	[[ -n "$version" ]] || die "${VERSION_FILE} must not be empty"
	[[ "$version" == "${lines[0]}" ]] || die "${VERSION_FILE} must not contain whitespace"
	validate_semver "$version"
	printf '%s' "$version"
}

validate_changelog() {
	local version="$1"
	local heading

	[[ -f "$CHANGELOG_FILE" ]] || die "missing ${CHANGELOG_FILE}"
	heading="$(grep -m1 '^## ' "$CHANGELOG_FILE" || true)"
	[[ -n "$heading" ]] || die "${CHANGELOG_FILE} must start with a version heading"
	[[ "$heading" == "## ${version} ("* ]] || \
		die "${CHANGELOG_FILE} first version heading must match ${VERSION_FILE}: expected '${version}', got '${heading}'"
}

validate_expected_version() {
	local version="$1"
	local expected="$EXPECTED_VERSION"

	[[ -n "$expected" ]] || return 0

	expected="${expected//[[:space:]]/}"
	[[ -n "$expected" ]] || die "expected version must not be empty"
	expected="${expected#v}"
	validate_semver "$expected"

	if [[ "$version" != "$expected" ]]; then
		die "${VERSION_FILE} version ${version} must match expected version ${expected}"
	fi
}

validate_tag_precedence() {
	local version="$1"
	local tag stripped highest candidate
	local -a versions=()

	git rev-parse --git-dir >/dev/null 2>&1 || return 0

	while IFS= read -r tag; do
		stripped="${tag#v}"
		validate_semver "$stripped"
		versions+=("$stripped")
	done < <(git tag --list 'v[0-9]*.[0-9]*.[0-9]*')

	if ((${#versions[@]} == 0)); then
		return 0
	fi

	highest="${versions[0]}"
	for candidate in "${versions[@]:1}"; do
		if semver_gt "$candidate" "$highest"; then
			highest="$candidate"
		fi
	done

	if ! semver_gt "$version" "$highest"; then
		die "${VERSION_FILE} version ${version} must be greater than highest existing tag v${highest}"
	fi
}

version="$(read_release_version)"
validate_changelog "$version"
validate_expected_version "$version"
if [[ "$CHECK_TAG_PRECEDENCE" == "1" ]]; then
	validate_tag_precedence "$version"
fi

if [[ "$GITHUB_OUTPUT_MODE" == "1" ]]; then
	[[ -n "${GITHUB_OUTPUT:-}" ]] || die "GITHUB_OUTPUT is required with --github-output"
	{
		echo "version=v${version}"
		echo "version_no_v=${version}"
	} >>"$GITHUB_OUTPUT"
fi

echo "Validated release version v${version}."
