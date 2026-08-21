#!/usr/bin/env bash

set -euo pipefail

script_directory=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
unit_source="$script_directory/peerblade-agent.service"
temporary_directory=$(mktemp -d)
temporary_unit="$temporary_directory/peerblade-agent.service"

cleanup() {
  rm -rf -- "$temporary_directory"
}
trap cleanup EXIT

command -v systemd-analyze >/dev/null 2>&1 || {
  echo "systemd-analyze is required" >&2
  exit 1
}

bash -n \
  "$script_directory/install.sh" \
  "$script_directory/quick-install.sh" \
  "$script_directory/quick-reconnect.sh" \
  "$script_directory/quick-uninstall.sh"

# The real installer verifies the installed unit after the agent binary exists.
# Repository verification substitutes only ExecStart so systemd-analyze can
# validate the remaining unit directives on a development machine.
sed \
  's#ExecStart=/usr/local/bin/peerblade-agent run#ExecStart=/bin/true#' \
  "$unit_source" >"$temporary_unit"

verification_output=""
if ! verification_output=$(systemd-analyze verify "$temporary_unit" 2>&1); then
  echo "$verification_output" >&2
  exit 1
fi

if grep -Eq '(^|/)peerblade-agent\.service:' <<<"$verification_output"; then
  echo "$verification_output" >&2
  exit 1
fi

echo "systemd unit verification passed"
