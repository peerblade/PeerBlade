#!/usr/bin/env bash

set -euo pipefail

script_directory=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
environment_file="$script_directory/.env"
setup_url_file="$script_directory/setup-url.txt"

fail() {
  echo "peerblade production bootstrap: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

main() {
  [[ $# -eq 1 ]] || fail "usage: sudo ./deploy/production/bootstrap.sh DOMAIN"
  [[ ${EUID:-$(id -u)} -eq 0 ]] || fail "run this command as root"

  local domain=$1
  local postgres_password

  [[ "$domain" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]] || \
    fail "DOMAIN has an invalid format"
  [[ "$domain" == *.* ]] || fail "DOMAIN must be a fully-qualified DNS name"
  [[ ! -e "$environment_file" ]] || fail "$environment_file already exists"
  [[ ! -e "$setup_url_file" ]] || fail "$setup_url_file already exists"

  require_command docker
  require_command openssl
  docker compose version >/dev/null
  docker network inspect peerblade-edge >/dev/null 2>&1 || \
    docker network create peerblade-edge >/dev/null

  postgres_password=$(openssl rand -hex 32)

  umask 077

  {
    printf 'PEERBLADE_DOMAIN=%s\n' "$domain"
    printf 'PEERBLADE_MARKETING_DOMAIN=peerblade.com\n'
    printf 'PEERBLADE_DEV_DOMAIN=dev.peerblade.com\n'
    printf 'PEERBLADE_API_HOSTNAME=peerblade-api\n'
    printf 'PEERBLADE_WEB_HOSTNAME=peerblade-web\n'
    printf 'PEERBLADE_AUTH_PROXY_MODE=app\n'
    printf 'PEERBLADE_IMAGE_TAG=latest\n'
    printf '\n'
    printf 'POSTGRES_DB=peerblade\n'
    printf 'POSTGRES_USER=peerblade\n'
    printf 'POSTGRES_PASSWORD=%s\n' "$postgres_password"
    printf 'LOG_LEVEL=info\n'
    printf 'AGENT_OFFLINE_AFTER_SECONDS=90\n'
    printf 'AGENT_ENROLLMENT_TTL_MINUTES=15\n'
    printf 'AUTH_SESSION_TTL_HOURS=168\n'
  } >"$environment_file"

  chmod 0600 "$environment_file"
  "$script_directory/setup-token.sh" "$domain"

  echo "Production secrets created"
  echo "Environment: $environment_file"
  echo "One-time setup URL: $setup_url_file"
}

main "$@"
