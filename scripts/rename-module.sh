#!/usr/bin/env bash
#
# Renames the Go module, e.g. when the product gets its final name:
#
#   scripts/rename-module.sh github.com/<org>/<repository>
#
# This rewrites go.mod and every import, then builds and tests to prove the
# rename is complete.

set -euo pipefail

new="${1:?usage: $0 <new-module-path>}"
cd "$(dirname "$0")/.."
old="$(go list -m)"

if [[ "$old" == "$new" ]]; then
  echo "module is already $new"
  exit 0
fi

go mod edit -module "$new"
grep -rl --include='*.go' "\"$old" . | while read -r file; do
  sed -i.bak "s#\"$old#\"$new#g" "$file" && rm "$file.bak"
done

go build ./...
go test ./...
echo "renamed $old -> $new"
echo "remaining mentions of the old path (docs, CI) to review:"
grep -rn --exclude-dir=.git "$old" . || echo "  none"
