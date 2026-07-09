#!/usr/bin/env bash

set -euo pipefail

version="$1"
provider="terraform-provider-ona"
timestamp="${SOURCE_DATE_EPOCH:-0}"

if [[ -z "$version" ]]; then
	echo "version is required" >&2
	exit 1
fi

mkdir -p linux_amd64 linux_arm64
cp "${BINARY_LINUX_AMD64:-build/linux_amd64/terraform-provider-ona}" "linux_amd64/${provider}_v${version}"
cp "${BINARY_LINUX_ARM64:-build/linux_arm64/terraform-provider-ona}" "linux_arm64/${provider}_v${version}"
chmod +x "linux_amd64/${provider}_v${version}" "linux_arm64/${provider}_v${version}"
touch -d "@${timestamp}" "linux_amd64/${provider}_v${version}" "linux_arm64/${provider}_v${version}"

(cd linux_amd64 && zip -q -9 -X "../${provider}_${version}_linux_amd64.zip" "${provider}_v${version}")
(cd linux_arm64 && zip -q -9 -X "../${provider}_${version}_linux_arm64.zip" "${provider}_v${version}")
cp "${REGISTRY_MANIFEST:-terraform-registry-manifest.json}" "${provider}_${version}_manifest.json"
touch -d "@${timestamp}" "${provider}_${version}_manifest.json"

sha256sum \
	"${provider}_${version}_linux_amd64.zip" \
	"${provider}_${version}_linux_arm64.zip" \
	"${provider}_${version}_manifest.json" \
	> "${provider}_${version}_SHA256SUMS"

rm -rf linux_amd64 linux_arm64 build terraform-registry-manifest.json
