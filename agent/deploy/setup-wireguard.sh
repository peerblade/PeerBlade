#!/usr/bin/env bash

set -euo pipefail

fail() {
  echo "peerblade WireGuard setup: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

validate_inputs() {
  [[ $# -eq 4 ]] || \
    fail "usage: sudo ./agent/deploy/setup-wireguard.sh INTERFACE ADDRESS_CIDR LISTEN_PORT OUTBOUND_INTERFACE"

  local interface_name=$1
  local address_cidr=$2
  local listen_port=$3
  local outbound_interface=$4
  local address=${address_cidr%/24}
  local first_octet
  local second_octet
  local third_octet
  local fourth_octet
  local octet
  local octet_value

  [[ "$interface_name" =~ ^[A-Za-z0-9_.-]{1,15}$ ]] || \
    fail "INTERFACE is not a valid Linux interface name"
  [[ "$address_cidr" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}/24$ ]] || \
    fail "ADDRESS_CIDR must be an IPv4 /24 address, for example 10.44.0.1/24"
  IFS=. read -r first_octet second_octet third_octet fourth_octet <<<"$address"
  for octet in "$first_octet" "$second_octet" "$third_octet" "$fourth_octet"; do
    octet_value=$((10#$octet))
    (( octet_value >= 0 && octet_value <= 255 )) || fail "ADDRESS_CIDR contains an invalid IPv4 address"
  done
  (( 10#$fourth_octet == 1 )) || \
    fail "ADDRESS_CIDR must use the first host address (.1/24)"
  [[ "$listen_port" =~ ^[0-9]+$ ]] && (( listen_port >= 1 && listen_port <= 65535 )) || \
    fail "LISTEN_PORT must be between 1 and 65535"
  [[ "$outbound_interface" =~ ^[A-Za-z0-9_.-]{1,15}$ ]] || \
    fail "OUTBOUND_INTERFACE is not a valid Linux interface name"
  ip link show dev "$outbound_interface" >/dev/null 2>&1 || \
    fail "outbound interface does not exist: $outbound_interface"
}

main() {
  [[ ${EUID:-$(id -u)} -eq 0 ]] || fail "run this command as root"
  require_command install
  require_command ip
  require_command iptables
  require_command systemctl
  require_command sysctl
  require_command wg
  require_command wg-quick
  validate_inputs "$@"

  local interface_name=$1
  local address_cidr=$2
  local listen_port=$3
  local outbound_interface=$4
  local address=${address_cidr%/24}
  local network_cidr=${address%.*}.0/24
  local config_path=/etc/wireguard/${interface_name}.conf
  local temporary_directory
  local temporary_config
  local private_key

  [[ ! -e "$config_path" ]] || fail "configuration already exists: $config_path"
  ! ip link show dev "$interface_name" >/dev/null 2>&1 || \
    fail "interface already exists: $interface_name"

  temporary_directory=$(mktemp -d)
  trap 'rm -rf -- "${temporary_directory:-}"' EXIT
  temporary_config=$temporary_directory/${interface_name}.conf
  private_key=$(wg genkey)

  umask 077
  {
    printf '[Interface]\n'
    printf 'Address = %s\n' "$address_cidr"
    printf 'ListenPort = %s\n' "$listen_port"
    printf 'PrivateKey = %s\n' "$private_key"
    printf 'PostUp = iptables -A FORWARD -i %%i -j ACCEPT; iptables -A FORWARD -o %%i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; iptables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE\n' "$network_cidr" "$outbound_interface"
    printf 'PostDown = iptables -D FORWARD -i %%i -j ACCEPT; iptables -D FORWARD -o %%i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; iptables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE\n' "$network_cidr" "$outbound_interface"
  } >"$temporary_config"

  install -d -m 0700 -o root -g root /etc/wireguard
  install -m 0600 -o root -g root "$temporary_config" "$config_path"
  install -d -m 0755 -o root -g root /etc/sysctl.d
  printf 'net.ipv4.ip_forward = 1\n' >/etc/sysctl.d/99-peerblade-wireguard.conf
  sysctl --system >/dev/null
  systemctl enable --now "wg-quick@${interface_name}.service"

  echo "PeerBlade WireGuard interface $interface_name is running on UDP $listen_port"
  echo "Open UDP $listen_port in the VPS firewall and set PEERBLADE_MANAGED_ENDPOINT to the public host and this port"
}

main "$@"
