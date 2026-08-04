#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
updater="${script_dir}/../token3-updater.sh"

same_output="$(bash "$updater" --dry-run --current 0.1.170-token3.2 --target 0.1.170)"
[[ "$same_output" == *"already includes official v0.1.170"* ]]

newer_output="$(bash "$updater" --dry-run --current 0.1.170-token3.2 --target 0.1.171)"
[[ "$newer_output" == *"can update from official v0.1.171"* ]]

if bash "$updater" --dry-run --current 0.1.170-token3.2 --target '0.1.171-rc1' >/dev/null 2>&1; then
  echo "prerelease target was accepted" >&2
  exit 1
fi

echo "token3 updater tests passed"
