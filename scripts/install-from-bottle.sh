#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <formula> <bottle-json>..." >&2
  exit 1
fi

formula="$1"
shift

# Homebrew trusts a tap formula when it is installed by its full name, but the
# merge below reads it first.
brew trust --formula "$formula"
brew bottle --merge --write --no-commit --no-all-checks "$@"
brew uninstall "$formula" 2>/dev/null || true
brew install "$formula"
brew test "$formula"

# A formula whose bottle cannot be used still installs, by compiling: only the
# install receipt tells the two apart.
poured="$(brew info --json=v2 --formula "$formula" | jq -r '.formulae[0].installed[0].poured_from_bottle')"
if [[ "$poured" != "true" ]]; then
  echo "$formula was compiled instead of installed from its bottle" >&2
  exit 1
fi
