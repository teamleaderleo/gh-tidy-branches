#!/usr/bin/env bash
set -euo pipefail

version="${1:?usage: script/build-release.sh VERSION}"
platforms=(
  darwin-amd64
  darwin-arm64
  linux-amd64
  linux-arm64
  windows-amd64
)

if [[ -n "${TIDY_BRANCHES_RELEASE_PLATFORMS:-}" ]]; then
  read -r -a platforms <<<"${TIDY_BRANCHES_RELEASE_PLATFORMS}"
fi

rm -rf dist
mkdir -p dist

for platform in "${platforms[@]}"; do
  goos="${platform%-*}"
  goarch="${platform#*-}"
  extension=""
  if [[ "$goos" == "windows" ]]; then
    extension=".exe"
  fi

  output="dist/${platform}${extension}"
  echo "Building ${output}"
  GOOS="$goos" \
    GOARCH="$goarch" \
    CGO_ENABLED=0 \
    go build \
      -trimpath \
      -ldflags="-s -w -X=main.version=${version}" \
      -o "$output" \
      ./cmd/gh-tidy-branches
done
