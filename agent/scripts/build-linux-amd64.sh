#!/usr/bin/env bash

set -euo pipefail

script_directory=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
agent_directory=$(cd -- "$script_directory/.." && pwd)
version=${PEERBLADE_AGENT_VERSION:-0.7.1}
output_directory="$agent_directory/dist"
output_file="$output_directory/peerblade-agent-linux-amd64"

if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.23 or newer is required" >&2
  exit 1
fi

if [[ ! "$version" =~ ^[0-9A-Za-z][0-9A-Za-z._+-]*$ ]]; then
  echo "PEERBLADE_AGENT_VERSION contains unsupported characters" >&2
  exit 1
fi

mkdir -p -- "$output_directory"

(
  cd -- "$agent_directory"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags "-s -w -X main.agentVersion=$version" \
    -o "$output_file" \
    .
)

(
  cd -- "$output_directory"
  sha256sum "$(basename -- "$output_file")" >"$(basename -- "$output_file").sha256"
)

echo "Built $output_file"
echo "Checksum: $output_file.sha256"
