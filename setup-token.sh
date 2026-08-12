#!/usr/bin/env bash
# Issues the one-time browser link used to create the first administrator.

set -euo pipefail

script_directory=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
environment_file="$script_directory/.env"
setup_url_file="$script_directory/setup-url.txt"
readonly setup_token_ttl_seconds=3600

fail() {
  echo "peerblade setup token: $*" >&2
  exit 1
}

main() {
  [[ ${EUID:-$(id -u)} -eq 0 ]] || fail "run this command as root"
  [[ -f "$environment_file" ]] || fail "$environment_file does not exist"
  command -v openssl >/dev/null 2>&1 || fail "openssl is required"

  local domain=${1:-}
  local setup_token
  local setup_token_hash
  local setup_token_expires_at
  local temporary_file

  if [[ -z "$domain" ]]; then
    domain=$(sed -n 's/^PEERBLADE_DOMAIN=//p' "$environment_file" | tail -n 1)
  fi

  [[ "$domain" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ && "$domain" == *.* ]] || \
    fail "DOMAIN must be a fully-qualified DNS name"

  setup_token=$(openssl rand -hex 32)
  setup_token_hash=$(printf '%s' "$setup_token" | openssl dgst -sha256 -r | awk '{print $1}')
  setup_token_expires_at=$(( $(date +%s) + setup_token_ttl_seconds ))
  temporary_file=$(mktemp "$script_directory/.env.setup-token.XXXXXX")
  trap 'rm -f -- "${temporary_file:-}"' EXIT

  grep -Ev '^AUTH_SETUP_TOKEN_(HASH|EXPIRES_AT)=' "$environment_file" >"$temporary_file"
  printf '\nAUTH_SETUP_TOKEN_HASH=%s\n' "$setup_token_hash" >>"$temporary_file"
  printf 'AUTH_SETUP_TOKEN_EXPIRES_AT=%s\n' "$setup_token_expires_at" >>"$temporary_file"
  chmod 0600 "$temporary_file"
  mv "$temporary_file" "$environment_file"
  trap - EXIT

  printf 'URL=https://%s/setup#token=%s\n' "$domain" "$setup_token" >"$setup_url_file"
  chmod 0600 "$setup_url_file"

  echo "One-time setup URL written to $setup_url_file (valid for 60 minutes)"
}

main "$@"
