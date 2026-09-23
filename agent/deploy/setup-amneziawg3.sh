#!/usr/bin/env bash

set -euo pipefail

fail() {
  echo "peerblade AmneziaWG 3.x setup: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

random_between() {
  local minimum=$1
  local maximum=$2
  local value
  value=$(od -An -N4 -tu4 /dev/urandom)
  echo $((minimum + value % (maximum - minimum + 1)))
}

main() {
  [[ ${EUID:-$(id -u)} -eq 0 ]] || fail "run this command as root"
  [[ $# -eq 4 ]] || fail "usage: setup-amneziawg3.sh INTERFACE ADDRESS_CIDR LISTEN_PORT OUTBOUND_INTERFACE"
  for command in awg awg-quick install ip iptables od systemctl sysctl; do
    require_command "$command"
  done

  local interface_name=$1
  local address_cidr=$2
  local listen_port=$3
  local outbound_interface=$4
  local address=${address_cidr%/24}
  local network_cidr=${address%.*}.0/24
  local config_directory=/etc/amnezia/amneziawg
  local config_path=$config_directory/${interface_name}.conf
  local temporary_directory
  local temporary_config
  local private_key
  local jc jmin jmax s1 hpk

  [[ "$interface_name" =~ ^[A-Za-z0-9_.-]{1,15}$ ]] || fail "invalid interface name"
  [[ "$address_cidr" =~ ^([0-9]{1,3}\.){3}1/24$ ]] || fail "ADDRESS_CIDR must be an IPv4 /24 ending in .1"
  [[ "$listen_port" =~ ^[0-9]+$ ]] && ((listen_port >= 1 && listen_port <= 65535)) || fail "invalid UDP port"
  ip link show dev "$outbound_interface" >/dev/null 2>&1 || fail "outbound interface does not exist"
  [[ ! -e "$config_path" ]] || fail "configuration already exists: $config_path"
  ! ip link show dev "$interface_name" >/dev/null 2>&1 || fail "interface already exists: $interface_name"

  jc=$(random_between 4 8)
  jmin=$(random_between 20 40)
  jmax=$(random_between 70 100)
  s1=$(random_between 12 64)
  hpk=$(awg genkey)

  temporary_directory=$(mktemp -d)
  trap 'rm -rf -- "${temporary_directory:-}"' EXIT
  temporary_config=$temporary_directory/${interface_name}.conf
  private_key=$(awg genkey)
  umask 077
  {
    printf '[Interface]\n'
    printf 'Address = %s\n' "$address_cidr"
    printf 'ListenPort = %s\n' "$listen_port"
    printf 'PrivateKey = %s\n' "$private_key"
    printf 'Jc = %s\nJmin = %s\nJmax = %s\n' "$jc" "$jmin" "$jmax"
    printf 'S1 = %s\nS2 = %s\nS3 = %s\nS4 = %s\n' "$s1" "$s1" "$s1" "$s1"
    printf 'H1 = 1\nH2 = 2\nH3 = 3\nH4 = 4\n'
    printf 'HeaderProtectionKey = %s\n' "$hpk"
    printf 'ContentPaddingAddition = 10-100\n'
    printf 'RekeyAfterTime = 100-120\nRekeyTimeout = 3-8\nRejectAfterTime = 150-180\n'
    printf 'KeepaliveTimeout = 7-13\nMaxHandshakeAttempts = 15-20\n'
    printf 'RandomTrailers = on\nDisableCookies = on\n'
    printf 'PostUp = iptables -A FORWARD -i %%i -j ACCEPT; iptables -A FORWARD -o %%i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; iptables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE\n' "$network_cidr" "$outbound_interface"
    printf 'PostDown = iptables -D FORWARD -i %%i -j ACCEPT; iptables -D FORWARD -o %%i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; iptables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE\n' "$network_cidr" "$outbound_interface"
  } >"$temporary_config"

  install -d -m 0700 -o root -g root "$config_directory"
  {
    printf 'PEERBLADE_AWG3_JC=%s\nPEERBLADE_AWG3_JMIN=%s\nPEERBLADE_AWG3_JMAX=%s\n' "$jc" "$jmin" "$jmax"
    printf 'PEERBLADE_AWG3_S1=%s\nPEERBLADE_AWG3_S2=%s\nPEERBLADE_AWG3_S3=%s\nPEERBLADE_AWG3_S4=%s\n' "$s1" "$s1" "$s1" "$s1"
    printf 'PEERBLADE_AWG3_H1=1\nPEERBLADE_AWG3_H2=2\nPEERBLADE_AWG3_H3=3\nPEERBLADE_AWG3_H4=4\n'
    printf 'PEERBLADE_AWG3_HEADER_PROTECTION_KEY=%s\n' "$hpk"
    printf 'PEERBLADE_AWG3_CONTENT_PADDING_ADDITION=10-100\nPEERBLADE_AWG3_REKEY_AFTER_TIME=100-120\nPEERBLADE_AWG3_REKEY_TIMEOUT=3-8\n'
    printf 'PEERBLADE_AWG3_REJECT_AFTER_TIME=150-180\nPEERBLADE_AWG3_KEEPALIVE_TIMEOUT=7-13\nPEERBLADE_AWG3_MAX_HANDSHAKE_ATTEMPTS=15-20\n'
    printf 'PEERBLADE_AWG3_RANDOM_TRAILERS=on\nPEERBLADE_AWG3_DISABLE_COOKIES=on\n'
  } >"$config_directory/.peerblade-awg3.env"
  chmod 0600 "$config_directory/.peerblade-awg3.env"
  install -m 0600 -o root -g root "$temporary_config" "$config_path"
  install -d -m 0755 -o root -g root /etc/sysctl.d
  printf 'net.ipv4.ip_forward = 1\n' >/etc/sysctl.d/99-peerblade-amneziawg3.conf
  sysctl --system >/dev/null
  systemctl enable --now "awg-quick@${interface_name}.service"

  awg show "$interface_name" >/dev/null || fail "interface did not become available"
  echo "PeerBlade AmneziaWG 3.x interface $interface_name is running on UDP $listen_port"
}

main "$@"
