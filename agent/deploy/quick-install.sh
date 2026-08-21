#!/usr/bin/env bash

set -euo pipefail
set +x

fail() {
  echo "PeerBlade installation failed: $*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  sudo bash install-agent.sh --url https://my.peerblade.com --token pen_... \
    [--endpoint vpn.example.com:51820] [--interface peerblade0] \
    [--subnet 10.44.0.1/24] [--dns 1.1.1.1,8.8.8.8] [--no-interface]

Supported: Ubuntu/Debian Linux on x86_64 or aarch64 with systemd.

With --endpoint the installer also prepares a dedicated WireGuard interface
for PeerBlade and hands it to the agent. Interfaces it did not create are
never touched; --no-interface installs the agent alone.
EOF
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

control_plane_url=""
enrollment_token=""
managed_endpoint=""
managed_interface="peerblade0"
managed_subnet="10.44.0.1/24"
managed_dns="1.1.1.1,8.8.8.8"
manage_interface=true
listen_port=""
interface_exists=false

parse_arguments() {
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
      --endpoint)
        [[ $# -ge 2 ]] || fail "--endpoint requires a value"
        managed_endpoint=$2
        shift 2
        ;;
      --interface)
        [[ $# -ge 2 ]] || fail "--interface requires a value"
        managed_interface=$2
        shift 2
        ;;
      --subnet)
        [[ $# -ge 2 ]] || fail "--subnet requires a value"
        managed_subnet=$2
        shift 2
        ;;
      --dns)
        [[ $# -ge 2 ]] || fail "--dns requires a value"
        managed_dns=$2
        shift 2
        ;;
      --no-interface)
        manage_interface=false
        shift
        ;;
      --help|-h)
        usage
        exit 0
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

  if [[ -z "$managed_endpoint" ]]; then
    manage_interface=false
    return
  fi

  [[ "$managed_endpoint" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?:[0-9]{1,5}$ ]] || \
    fail "--endpoint must look like vpn.example.com:51820"
  [[ "$managed_interface" =~ ^[A-Za-z0-9_.-]{1,15}$ ]] || \
    fail "--interface is not a valid Linux interface name"
  [[ "$managed_subnet" =~ ^([0-9]{1,3}\.){3}1/24$ ]] || \
    fail "--subnet must be an IPv4 /24 ending in .1, for example 10.44.0.1/24"
  [[ "$managed_dns" =~ ^[0-9a-fA-F.:,]+$ ]] || fail "--dns must be a comma-separated list of addresses"

  listen_port=${managed_endpoint##*:}
  (( 10#$listen_port >= 1 && 10#$listen_port <= 65535 )) || \
    fail "--endpoint has an invalid port"
}

# Everything that could stop us runs before the one-time token is spent, so a
# host that is not ready costs an enrollment rather than half an installation.
prepare_interface_prerequisites() {
  local configuration_path=/etc/wireguard/${managed_interface}.conf

  local configured_port

  if [[ -e "$configuration_path" ]] || ip link show dev "$managed_interface" >/dev/null 2>&1; then
    interface_exists=true
    echo "Interface $managed_interface already exists; leaving it as it is."
    configured_port=$(sed -n 's/^[[:space:]]*ListenPort[[:space:]]*=[[:space:]]*\([0-9]\+\).*/\1/p' \
      "$configuration_path" 2>/dev/null | head -n 1)

    if [[ -n "$configured_port" && "$configured_port" != "$listen_port" ]]; then
      echo "Warning: $managed_interface listens on UDP $configured_port," \
        "while the endpoint advertises $listen_port" >&2
    fi

    return
  fi

  if ! command -v wg >/dev/null 2>&1 || ! command -v wg-quick >/dev/null 2>&1; then
    command -v apt-get >/dev/null 2>&1 || \
      fail "wireguard-tools is missing; install it and re-run, or pass --no-interface"
    echo "Installing wireguard-tools..."
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq wireguard-tools iptables >/dev/null
  fi

  require_command ip
  require_command iptables
  require_command sysctl

  if command -v ss >/dev/null 2>&1 && \
    ss -lun "( sport = :$listen_port )" 2>/dev/null | grep -q UNCONN; then
    fail "UDP $listen_port is already in use; re-run with --endpoint HOST:OTHER_PORT or --no-interface"
  fi
}

outbound_interface() {
  ip route show default 2>/dev/null | awk '/^default/ {for (i = 1; i < NF; i++) if ($i == "dev") {print $(i + 1); exit}}'
}

create_interface() {
  local outbound

  if [[ "$interface_exists" == true ]]; then
    return
  fi

  outbound=$(outbound_interface)
  [[ -n "$outbound" ]] || fail "cannot determine the outbound interface from the default route"

  echo "Preparing WireGuard interface $managed_interface on UDP $listen_port..."
  "$1" "$managed_interface" "$managed_subnet" "$listen_port" "$outbound"

  # ufw filters forwarded traffic separately, so an allowed port alone still
  # leaves peers without a route out.
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q '^Status: active'; then
    echo "Opening UDP $listen_port in ufw..."
    ufw allow "$listen_port/udp" >/dev/null
    ufw route allow in on "$managed_interface" out on "$outbound" >/dev/null
    ufw route allow in on "$outbound" out on "$managed_interface" >/dev/null
  else
    echo "Open UDP $listen_port in the VPS firewall if one runs outside this host."
  fi
}

main() {
  if [[ "${1:-}" == --help || "${1:-}" == -h ]]; then
    usage
    return
  fi

  [[ ${EUID:-$(id -u)} -eq 0 ]] || fail "run this command as root"

  parse_arguments "$@"

  [[ $(uname -s) == Linux ]] || fail "Linux is required"

  local agent_binary_name

  case "$(uname -m)" in
    x86_64)
      agent_binary_name="peerblade-agent-linux-amd64"
      ;;
    aarch64 | arm64)
      agent_binary_name="peerblade-agent-linux-arm64"
      ;;
    *)
      fail "unsupported architecture $(uname -m); x86_64 and aarch64 are supported"
      ;;
  esac

  [[ -d /run/systemd/system ]] || fail "systemd is required"

  require_command curl
  require_command install
  require_command sed
  require_command sha256sum
  require_command systemctl

  if command -v docker >/dev/null 2>&1 && \
    docker ps --format '{{.Names}}' 2>/dev/null | grep -qx 'peerblade-agent'; then
    fail "a Docker-based peerblade-agent is running; choose 'Reconnect installed agent' in PeerBlade or stop it before migrating to systemd"
  fi

  if [[ "$manage_interface" == true ]]; then
    prepare_interface_prerequisites
  fi

  local temporary_directory
  temporary_directory=$(mktemp -d)
  trap 'rm -rf -- "${temporary_directory:-}"' EXIT

  echo "Downloading and verifying PeerBlade agent..."
  curl --fail --silent --show-error --location \
    "$control_plane_url/downloads/$agent_binary_name" \
    --output "$temporary_directory/$agent_binary_name"
  curl --fail --silent --show-error --location \
    "$control_plane_url/downloads/$agent_binary_name.sha256" \
    --output "$temporary_directory/$agent_binary_name.sha256"
  curl --fail --silent --show-error --location \
    "$control_plane_url/downloads/install.sh" \
    --output "$temporary_directory/install.sh"
  curl --fail --silent --show-error --location \
    "$control_plane_url/downloads/peerblade-agent.service" \
    --output "$temporary_directory/peerblade-agent.service"

  (
    cd "$temporary_directory"
    sha256sum --check "$agent_binary_name.sha256"
  )
  chmod 0755 "$temporary_directory/$agent_binary_name" \
    "$temporary_directory/install.sh"

  if [[ "$manage_interface" == true && "$interface_exists" == false ]]; then
    curl --fail --silent --show-error --location \
      "$control_plane_url/downloads/setup-wireguard.sh" \
      --output "$temporary_directory/setup-wireguard.sh"
    chmod 0755 "$temporary_directory/setup-wireguard.sh"
    create_interface "$temporary_directory/setup-wireguard.sh"
  fi

  echo "Exchanging the one-time enrollment token..."
  local enrollment_response
  enrollment_response=$(
    printf '{"token":"%s"}' "$enrollment_token" | \
      curl --fail --silent --show-error \
        --request POST \
        --header 'Content-Type: application/json' \
        --data-binary @- \
        "$control_plane_url/api/agent/v1/enroll"
  ) || fail "the enrollment token is invalid, expired or already used"

  local server_id
  local agent_token
  server_id=$(printf '%s' "$enrollment_response" | \
    sed -n 's/.*"serverId"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  agent_token=$(printf '%s' "$enrollment_response" | \
    sed -n 's/.*"agentToken"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  unset enrollment_response enrollment_token

  [[ "$server_id" =~ ^[0-9a-fA-F-]{36}$ ]] || \
    fail "Control Plane returned an invalid server id"
  [[ "$agent_token" =~ ^pbl_[A-Za-z0-9_-]{43}$ ]] || \
    fail "Control Plane returned invalid agent credentials"

  local environment_file="$temporary_directory/agent.env"
  umask 077
  {
    printf 'PEERBLADE_API_URL=%s\n' "$control_plane_url"
    printf 'PEERBLADE_SERVER_ID=%s\n' "$server_id"
    printf 'PEERBLADE_AGENT_TOKEN=%s\n' "$agent_token"
    printf 'PEERBLADE_HEARTBEAT_INTERVAL=30s\n'
    printf 'PEERBLADE_SNAPSHOT_INTERVAL=60s\n'
    printf 'PEERBLADE_COMMAND_INTERVAL=5s\n'

    if [[ "$manage_interface" == true ]]; then
      printf 'PEERBLADE_MANAGED_INTERFACE=%s\n' "$managed_interface"
      printf 'PEERBLADE_MANAGED_ENDPOINT=%s\n' "$managed_endpoint"
      printf 'PEERBLADE_MANAGED_ADDRESS_CIDR=%s\n' "$managed_subnet"
      printf 'PEERBLADE_MANAGED_DNS=%s\n' "$managed_dns"
      printf 'PEERBLADE_MANAGED_ALLOWED_IPS=0.0.0.0/0\n'
      printf 'PEERBLADE_STATE_DIRECTORY=/var/lib/peerblade-agent\n'
    fi
  } >"$environment_file"
  unset agent_token

  "$temporary_directory/install.sh" install \
    "$temporary_directory/$agent_binary_name" \
    "$environment_file"

  if ! systemctl is-active --quiet peerblade-agent.service; then
    journalctl --unit peerblade-agent.service --no-pager --lines 20 >&2 || true
    fail "peerblade-agent.service did not start"
  fi

  echo

  if [[ "$manage_interface" == true ]]; then
    echo "PeerBlade agent is connected and manages $managed_interface on $managed_endpoint."
    echo "Return to the dashboard and add the first peer; status updates automatically."
  else
    echo "PeerBlade agent is connected. Return to the dashboard; status updates automatically."
  fi
}

main "$@"
