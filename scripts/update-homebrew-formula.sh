#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <git-tag> <formula-path>" >&2
  exit 1
fi

tag="$1"
formula_path="$2"

if [[ ! "$tag" =~ ^v.+ ]]; then
  echo "tag must start with v, got: $tag" >&2
  exit 1
fi

if [[ ! -f "$formula_path" ]]; then
  echo "formula not found: $formula_path" >&2
  exit 1
fi

formula_version="${tag#v}"
source_url="https://github.com/Cassidy321/jogai/archive/refs/tags/${tag}.tar.gz"

tmp_tarball="$(mktemp -t jogai-source.XXXXXX.tar.gz)"
tmp_formula="$(mktemp -t jogai-formula.XXXXXX.rb)"
trap 'rm -f "$tmp_tarball" "$tmp_formula"' EXIT

curl -fsSL "$source_url" -o "$tmp_tarball"
sha256="$(shasum -a 256 "$tmp_tarball" | awk '{print $1}')"

awk \
  -v source_url="$source_url" \
  -v formula_version="$formula_version" \
  -v sha256="$sha256" \
  '
  /^  url "/ {
    print "  url \"" source_url "\""
    next
  }
  /^  version "/ {
    print "  version \"" formula_version "\""
    next
  }
  /^  sha256 "/ {
    print "  sha256 \"" sha256 "\""
    next
  }
  {
    print
  }
  ' "$formula_path" > "$tmp_formula"

mv "$tmp_formula" "$formula_path"
