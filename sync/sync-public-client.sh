#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
source_root="${1:-${GITPOD_NEXT_DIR:-../gitpod-next}}"
if [[ "$source_root" != /* ]]; then
  source_root="$(cd "$repo_root" && cd "$source_root" && pwd)"
fi

src="$source_root/api/public-clients/go"
dest="$repo_root/api/public-clients/go"

if [[ ! -d "$src" ]]; then
  echo "missing source public Go client: $src" >&2
  exit 1
fi

rm -rf "$dest"
mkdir -p "$(dirname "$dest")"
cp -a "$src" "$dest"

go run "$repo_root/sync/rewrite-go-imports.go" \
  -root "$dest" \
  -old "github.com/gitpod-io/gitpod-next/api/go" \
  -new "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go"

find "$dest" -name '*.go' -print0 | xargs -0 gofmt -w
