#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <tap-checkout>" >&2
  exit 1
fi

tap_root="$(brew --repository)/Library/Taps/cassidy321"
mkdir -p "$tap_root"
ln -sfn "$1" "$tap_root/homebrew-tap"
