#!/bin/bash

echo "Generating mocks for all services..."

set -eou pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck disable=SC1091
source "$script_dir/../../../../dev/retry.sh"

# install mockgen
retry_command go install go.uber.org/mock/mockgen@v0.6.0

# make sure we're in the right directory
cd "$script_dir"

# generate mock for a single file
generate_mock_file() {
  local src_file="$1"
  local dest_dir="$2"

  local dest
  dest="$(basename "$src_file" .go)_mock.go"
  if [ "$dest_dir" != "." ]; then
    dest="$dest_dir/$dest"
  fi

  echo "Generating mock for $src_file"
  mockgen -source="$src_file" -destination="$dest" -package=mock
}

export -f generate_mock_file

# collect all files to process
declare -a FILES
declare -a DEST_DIRS

# v1 services (no prefix)
for f in ../../v1/v1connect/*.connect.go; do
  FILES+=("$f")
  DEST_DIRS+=(".")
done

# agentops services
mkdir -p agentops
for f in ../../agentops/v1/v1connect/*.connect.go; do
  FILES+=("$f")
  DEST_DIRS+=("agentops")
done

# run in parallel
# shellcheck disable=SC2016  # This is intentional: the single-quoted string is passed to xargs and evaluated as a command, so variable expansion happens at execution time.
for i in "${!FILES[@]}"; do
  printf "%s\0%s\0" "${FILES[$i]}" "${DEST_DIRS[$i]}"
done | xargs -0 --max-procs=16 --max-args=2 bash -c 'generate_mock_file "$1" "$2"' _

echo "Mock generation complete!"
