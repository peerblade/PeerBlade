#!/usr/bin/env bash

set -euo pipefail

readonly service_name="peerblade-agent.service"
readonly service_user="peerblade"
readonly binary_target="/usr/local/bin/peerblade-agent"
readonly config_directory="/etc/peerblade"
readonly config_target="$config_directory/agent.env"
readonly unit_target="/etc/systemd/system/$service_name"

script_directory=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
unit_source="$script_directory/peerblade-agent.service"

usage() {
  cat <<'EOF'
Usage:
  ./deploy/agent/install.sh check BINARY_PATH ENV_FILE
  sudo ./deploy/agent/install.sh install BINARY_PATH ENV_FILE
  sudo ./deploy/agent/install.sh uninstall

The uninstall action preserves /etc/peerblade/agent.env and the peerblade user.
EOF
}

fail() {
  echo "peerblade installer: $*" >&2
  exit 1
}

require_root() {
  [[ ${EUID:-$(id -u)} -eq 0 ]] || fail "run this command as root"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

validate_environment_file() {
  local environment_file=$1
  local api_url
  local server_id
  local token
  local managed_interface
  local managed_transport
  local variable

  [[ -f "$environment_file" ]] || fail "environment file does not exist: $environment_file"

  for variable in PEERBLADE_API_URL PEERBLADE_SERVER_ID PEERBLADE_AGENT_TOKEN; do
    [[ $(grep -Ec "^${variable}=.+$" "$environment_file") -eq 1 ]] || \
      fail "$environment_file must contain exactly one non-empty $variable"
  done

  api_url=$(sed -n 's/^PEERBLADE_API_URL=//p' "$environment_file")
  server_id=$(sed -n 's/^PEERBLADE_SERVER_ID=//p' "$environment_file")
  token=$(sed -n 's/^PEERBLADE_AGENT_TOKEN=//p' "$environment_file")

  [[ "$api_url" =~ ^https?://[^[:space:]]+$ ]] || \
    fail "PEERBLADE_API_URL must be an absolute HTTP or HTTPS URL"
  [[ "$server_id" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$ ]] || \
    fail "PEERBLADE_SERVER_ID must be a UUID v4"
  [[ "$token" =~ ^pbl_[A-Za-z0-9_-]{43}$ ]] || \
    fail "PEERBLADE_AGENT_TOKEN has an invalid format"

  managed_interface=$(sed -n 's/^PEERBLADE_MANAGED_INTERFACE=//p' "$environment_file")
  if [[ -n "$managed_interface" ]]; then
    for variable in PEERBLADE_MANAGED_ENDPOINT PEERBLADE_MANAGED_ADDRESS_CIDR PEERBLADE_STATE_DIRECTORY; do
      [[ $(grep -Ec "^${variable}=.+$" "$environment_file") -eq 1 ]] || \
        fail "$environment_file must contain exactly one non-empty $variable when native management is enabled"
    done
    [[ "$managed_interface" =~ ^[A-Za-z0-9_.-]{1,15}$ ]] || \
      fail "PEERBLADE_MANAGED_INTERFACE has an invalid Linux interface name"
    managed_transport=$(sed -n 's/^PEERBLADE_MANAGED_TRANSPORT=//p' "$environment_file")
    [[ -z "$managed_transport" || "$managed_transport" == wireguard || "$managed_transport" == amneziawg ]] || \
      fail "PEERBLADE_MANAGED_TRANSPORT must be wireguard or amneziawg"
    if [[ "$managed_transport" == amneziawg ]]; then
      for variable in PEERBLADE_AWG_JC PEERBLADE_AWG_JMIN PEERBLADE_AWG_JMAX PEERBLADE_AWG_S1 PEERBLADE_AWG_S2 PEERBLADE_AWG_H1 PEERBLADE_AWG_H2 PEERBLADE_AWG_H3 PEERBLADE_AWG_H4; do
        [[ $(grep -Ec "^${variable}=[0-9]+$" "$environment_file") -eq 1 ]] || \
          fail "$environment_file must contain one numeric $variable for AmneziaWG"
      done
    fi
  fi

  if [[ "$api_url" == http://* ]]; then
    echo "peerblade installer: warning: use HTTPS outside an isolated test network" >&2
  fi
}

check_inputs() {
  [[ $# -eq 2 ]] || {
    usage
    exit 2
  }

  local binary_source=$1
  local environment_source=$2

  [[ -f "$binary_source" ]] || fail "binary does not exist: $binary_source"
  [[ -x "$binary_source" ]] || fail "binary is not executable: $binary_source"
  [[ -f "$unit_source" ]] || fail "unit file does not exist: $unit_source"
  "$binary_source" version >/dev/null || \
    fail "binary cannot run on this host"
  validate_environment_file "$environment_source"
}

install_agent() {
  [[ $# -eq 2 ]] || {
    usage
    exit 2
  }

  local binary_source=$1
  local environment_source=$2
  local nologin_shell

  check_inputs "$binary_source" "$environment_source"

  if ! getent passwd "$service_user" >/dev/null; then
    nologin_shell=$(command -v nologin || true)
    [[ -n "$nologin_shell" ]] || nologin_shell=/usr/sbin/nologin
    useradd \
      --system \
      --no-create-home \
      --home-dir /nonexistent \
      --shell "$nologin_shell" \
      "$service_user"
  fi

  if systemctl is-active --quiet "$service_name"; then
    systemctl stop "$service_name"
  fi

  install -D -m 0755 -o root -g root "$binary_source" "$binary_target"
  install -d -m 0755 -o root -g root "$config_directory"
  install -m 0600 -o root -g root "$environment_source" "$config_target"
  install -D -m 0644 -o root -g root "$unit_source" "$unit_target"

  systemd-analyze verify "$unit_target"
  systemctl daemon-reload
  systemctl enable --now "$service_name"

  echo "PeerBlade agent installed and started"
  echo "Check it with: systemctl status $service_name"
}

uninstall_agent() {
  [[ $# -eq 0 ]] || {
    usage
    exit 2
  }

  if [[ -f "$unit_target" ]]; then
    systemctl disable --now "$service_name" || true
  fi

  rm -f -- "$unit_target" "$binary_target"
  systemctl daemon-reload
  systemctl reset-failed "$service_name" || true

  echo "PeerBlade agent removed"
  echo "Preserved configuration: $config_target"
  echo "Preserved system user: $service_user"
}

main() {
  [[ $# -ge 1 ]] || {
    usage
    exit 2
  }

  local action=$1
  shift

  case "$action" in
    check)
      check_inputs "$@"
      echo "Agent binary and environment file are valid"
      ;;
    install)
      require_root
      require_command getent
      require_command install
      require_command systemctl
      require_command systemd-analyze
      require_command useradd
      install_agent "$@"
      ;;
    uninstall)
      require_root
      require_command systemctl
      uninstall_agent "$@"
      ;;
    *)
      usage
      exit 2
      ;;
  esac
}

main "$@"
