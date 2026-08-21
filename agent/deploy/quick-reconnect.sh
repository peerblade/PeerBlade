#!/usr/bin/env bash

set -euo pipefail
set +x

readonly service_name="peerblade-agent.service"
readonly unit_path="/etc/systemd/system/$service_name"
readonly binary_path="/usr/local/bin/peerblade-agent"
readonly environment_path="/etc/peerblade/agent.env"
readonly container_name="peerblade-agent"

fail() {
  echo "PeerBlade reconnection failed: $*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  sudo bash reconnect-agent.sh --url https://my.peerblade.com --token pen_...

Re-enrolls an existing systemd or Docker Compose PeerBlade agent.
The agent binary, WireGuard interfaces, peers and local state are preserved.
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

assert_control_plane() {
  local source_name=$1
  local actual_url=${2%/}
  local expected_url=$3

  [[ "$actual_url" == "$expected_url" ]] || \
    fail "$source_name belongs to $actual_url, not $expected_url"
}

replace_credentials() {
  local target_file=$1
  local server_id=$2
  local agent_token=$3
  local temporary_file

  [[ "$target_file" != *,* ]] || \
    fail "multiple Docker Compose environment files are not supported"
  [[ -f "$target_file" ]] || fail "environment file is missing: $target_file"

  temporary_file=$(mktemp "${target_file}.peerblade-reconnect.XXXXXX")
  grep -Ev '^(PEERBLADE_SERVER_ID|PEERBLADE_AGENT_TOKEN)=' \
    "$target_file" >"$temporary_file" || true
  {
    printf 'PEERBLADE_SERVER_ID=%s\n' "$server_id"
    printf 'PEERBLADE_AGENT_TOKEN=%s\n' "$agent_token"
  } >>"$temporary_file"
  install -m 0600 -o root -g root "$temporary_file" "$target_file"
  rm -f -- "$temporary_file"
}

exchange_enrollment() {
  local control_plane_url=$1
  local enrollment_token=$2

  printf '{"token":"%s"}' "$enrollment_token" | \
    curl --fail --silent --show-error \
      --request POST \
      --header 'Content-Type: application/json' \
      --data-binary @- \
      "$control_plane_url/api/agent/v1/enroll"
}

main() {
  [[ ${EUID:-$(id -u)} -eq 0 ]] || fail "run this command as root"

  local control_plane_url=""
  local enrollment_token=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --url)
        [[ $# -ge 2 ]] || fail "--url requires a value"
        control_plane_url=${2%/}
        shift 2
        ;;
      --token)
        [[ $# -ge 2 ]] || fail "--token requires a value"
        enrollment_token=$2
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
  [[ "$enrollment_token" =~ ^pen_[A-Za-z0-9_-]{43}$ ]] || \
    fail "--token has an invalid format"
  [[ $(uname -s) == Linux ]] || fail "Linux is required"

  require_command curl
  require_command grep
  require_command install
  require_command mktemp
  require_command sed
  require_command tail

  local systemd_found=false
  local docker_found=false
  local systemd_environment=""
  local docker_environment=""
  local docker_environment_file=""
  local docker_compose_file=""
  local docker_project=""
  local docker_service=""

  if [[ -f "$environment_path" ]]; then
    [[ -f "$unit_path" && -x "$binary_path" ]] || \
      fail "systemd agent files are incomplete; use full installation"
    require_command systemctl
    systemd_environment=$(<"$environment_path")
    assert_control_plane \
      "systemd agent" \
      "$(read_environment_value PEERBLADE_API_URL "$systemd_environment")" \
      "$control_plane_url"
    systemd_found=true
  fi

  if command -v docker >/dev/null 2>&1 && \
    docker ps --all --format '{{.Names}}' | grep -qx "$container_name"; then
    docker_environment=$(
      docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' \
        "$container_name"
    )
    assert_control_plane \
      "Docker agent" \
      "$(read_environment_value PEERBLADE_API_URL "$docker_environment")" \
      "$control_plane_url"
    docker_environment_file=$(
      docker inspect --format \
        '{{index .Config.Labels "com.docker.compose.project.environment_file"}}' \
        "$container_name"
    )
    docker_compose_file=$(
      docker inspect --format \
        '{{index .Config.Labels "com.docker.compose.project.config_files"}}' \
        "$container_name"
    )
    docker_project=$(
      docker inspect --format \
        '{{index .Config.Labels "com.docker.compose.project"}}' \
        "$container_name"
    )
    docker_service=$(
      docker inspect --format \
        '{{index .Config.Labels "com.docker.compose.service"}}' \
        "$container_name"
    )
    [[ -n "$docker_environment_file" && -n "$docker_compose_file" && \
      -n "$docker_project" && -n "$docker_service" ]] || \
      fail "the Docker agent is not managed by Compose; use full installation"
    [[ "$docker_compose_file" != *,* ]] || \
      fail "multiple Docker Compose files are not supported"
    [[ -f "$docker_environment_file" && -f "$docker_compose_file" ]] || \
      fail "Docker Compose files are missing; use full installation"
    docker compose version >/dev/null || fail "Docker Compose v2 is required"
    docker_found=true
  fi

  if [[ "$systemd_found" == true && "$docker_found" == true ]]; then
    fail "both systemd and Docker agents exist; remove one before reconnecting"
  fi
  if [[ "$systemd_found" == false && "$docker_found" == false ]]; then
    fail "no installed PeerBlade agent was found; use full installation"
  fi

  echo "Exchanging the one-time enrollment token..."
  local enrollment_response
  enrollment_response=$(
    exchange_enrollment "$control_plane_url" "$enrollment_token"
  ) || fail "the enrollment token is invalid, expired or already used"
  unset enrollment_token

  local server_id
  local agent_token
  server_id=$(printf '%s' "$enrollment_response" | \
    sed -n 's/.*"serverId"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  agent_token=$(printf '%s' "$enrollment_response" | \
    sed -n 's/.*"agentToken"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  unset enrollment_response

  [[ "$server_id" =~ ^[0-9a-fA-F-]{36}$ ]] || \
    fail "Control Plane returned an invalid server id"
  [[ "$agent_token" =~ ^pbl_[A-Za-z0-9_-]{43}$ ]] || \
    fail "Control Plane returned invalid agent credentials"

  if [[ "$systemd_found" == true ]]; then
    replace_credentials "$environment_path" "$server_id" "$agent_token"
    unset agent_token
    systemctl restart "$service_name"
    systemctl is-active --quiet "$service_name" || \
      fail "$service_name did not start"
    echo "Existing systemd agent reconnected."
  else
    replace_credentials "$docker_environment_file" "$server_id" "$agent_token"
    unset agent_token
    docker compose \
      --env-file "$docker_environment_file" \
      --project-name "$docker_project" \
      --file "$docker_compose_file" \
      up --detach --force-recreate "$docker_service"
    echo "Existing Docker Compose agent reconnected."
  fi

  echo "The agent binary, WireGuard and local state were not changed."
  echo "Return to PeerBlade; status updates automatically."
}

main "$@"
