#!/usr/bin/env bash

set -euo pipefail
set +x

readonly service_name="peerblade-agent.service"
readonly unit_path="/etc/systemd/system/$service_name"
readonly binary_path="/usr/local/bin/peerblade-agent"
readonly environment_path="/etc/peerblade/agent.env"
readonly state_directory="/var/lib/peerblade-agent"
readonly container_name="peerblade-agent"

fail() {
  echo "PeerBlade removal failed: $*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  sudo bash uninstall-agent.sh --url https://my.peerblade.com --server-id UUID

Removes only the matching PeerBlade agent and its credentials.
WireGuard interfaces, peers and /var/lib/peerblade-agent are preserved.
EOF
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

read_environment_value() {
  local name=$1
  local content=$2
  printf '%s\n' "$content" | sed -n "s/^${name}=//p" | tail -n 1
}

remove_agent_credentials_from_file() {
  local environment_file=$1
  local temporary_file

  [[ "$environment_file" != *,* ]] || \
    fail "multiple Docker Compose environment files require manual cleanup"
  [[ -f "$environment_file" ]] || \
    fail "Docker Compose environment file is missing: $environment_file"
  require_command install
  require_command mktemp

  temporary_file=$(mktemp "${environment_file}.peerblade-cleanup.XXXXXX")
  if ! grep -Ev '^(PEERBLADE_SERVER_ID|PEERBLADE_AGENT_TOKEN)=' \
    "$environment_file" >"$temporary_file"; then
    [[ -s "$temporary_file" ]] || {
      rm -f -- "$temporary_file"
      fail "refusing to replace an empty Docker Compose environment file"
    }
  fi
  install -m 0600 -o root -g root "$temporary_file" "$environment_file"
  rm -f -- "$temporary_file"
}

assert_identity() {
  local source_name=$1
  local actual_url=${2%/}
  local actual_server_id=$3
  local expected_url=$4
  local expected_server_id=$5

  [[ "$actual_url" == "$expected_url" ]] || \
    fail "$source_name belongs to $actual_url, not $expected_url"
  [[ "$actual_server_id" == "$expected_server_id" ]] || \
    fail "$source_name belongs to another PeerBlade server"
}

main() {
  [[ ${EUID:-$(id -u)} -eq 0 ]] || fail "run this command as root"

  local control_plane_url=""
  local server_id=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --url)
        [[ $# -ge 2 ]] || fail "--url requires a value"
        control_plane_url=${2%/}
        shift 2
        ;;
      --server-id)
        [[ $# -ge 2 ]] || fail "--server-id requires a value"
        server_id=$2
        shift 2
        ;;
      --help|-h)
        usage
        return
        ;;
      *)
        fail "unknown argument: $1"
        ;;
    esac
  done

  [[ "$control_plane_url" =~ ^https://[^[:space:]]+$ ]] || \
    fail "--url must be an HTTPS URL"
  [[ "$server_id" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$ ]] || \
    fail "--server-id must be a UUID v4"
  [[ $(uname -s) == Linux ]] || fail "Linux is required"

  require_command grep
  require_command sed
  require_command tail

  local systemd_found=false
  local docker_found=false
  local systemd_environment=""
  local docker_environment=""
  local docker_environment_file=""

  if [[ -f "$environment_path" || -f "$unit_path" || -f "$binary_path" ]]; then
    [[ -f "$environment_path" ]] || \
      fail "systemd agent files are incomplete: $environment_path is missing"
    systemd_environment=$(<"$environment_path")
    assert_identity \
      "systemd agent" \
      "$(read_environment_value PEERBLADE_API_URL "$systemd_environment")" \
      "$(read_environment_value PEERBLADE_SERVER_ID "$systemd_environment")" \
      "$control_plane_url" \
      "$server_id"
    systemd_found=true
  fi

  if command -v docker >/dev/null 2>&1 && \
    docker ps --all --format '{{.Names}}' | grep -qx "$container_name"; then
    docker_environment=$(
      docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' \
        "$container_name"
    )
    docker_environment_file=$(
      docker inspect --format \
        '{{index .Config.Labels "com.docker.compose.project.environment_file"}}' \
        "$container_name"
    )
    assert_identity \
      "Docker agent" \
      "$(read_environment_value PEERBLADE_API_URL "$docker_environment")" \
      "$(read_environment_value PEERBLADE_SERVER_ID "$docker_environment")" \
      "$control_plane_url" \
      "$server_id"
    docker_found=true
  fi

  if [[ "$systemd_found" == true && "$docker_found" == true ]]; then
    fail "both systemd and Docker agents match; remove one manually first"
  fi
  if [[ "$systemd_found" == false && "$docker_found" == false ]]; then
    fail "no matching PeerBlade agent was found"
  fi

  if [[ "$systemd_found" == true ]]; then
    require_command systemctl
    systemctl disable --now "$service_name" || true
    rm -f -- "$unit_path" "$binary_path" "$environment_path"
    systemctl daemon-reload
    systemctl reset-failed "$service_name" || true
    echo "PeerBlade systemd agent and credentials removed."
    echo "Preserved agent state: $state_directory"
  else
    if docker ps --format '{{.Names}}' | grep -qx "$container_name"; then
      docker stop "$container_name" >/dev/null
    fi
    docker rm "$container_name" >/dev/null
    if [[ -n "$docker_environment_file" ]]; then
      remove_agent_credentials_from_file "$docker_environment_file"
      echo "Removed agent credentials from: $docker_environment_file"
    fi
    echo "PeerBlade Docker agent removed. Docker volumes were preserved."
  fi

  echo "WireGuard interfaces and peers were not changed."
  echo "Return to PeerBlade and confirm deletion from the Control Plane."
}

main "$@"
