#!/usr/bin/env bash
# Uptime Kuma control for hosts whose public port changes after a stop/start.
#
# The published port lives in .env, so the recovery after a restart is:
#   ./uk.sh port <new-port>
# That rewrites UK_PORT, recreates the container when Docker is reachable, and
# prints the new URL. `./uk.sh url` prints the address currently in use.
set -euo pipefail
cd "$(dirname "$0")"

ENV_FILE=.env
COMPOSE=(docker compose -f docker-compose.yml --env-file "$ENV_FILE")

say() { printf '%s\n' "$*"; }
die() { printf 'uk: %s\n' "$*" >&2; exit 1; }

require_env() {
  [ -f "$ENV_FILE" ] || die "missing $ENV_FILE — run: cp .env.example $ENV_FILE"
}

read_env() { # key -> value (last match wins, empty when unset)
  [ -f "$ENV_FILE" ] || return 0
  sed -n "s/^[[:space:]]*$1[[:space:]]*=//p" "$ENV_FILE" | tail -n 1
}

set_env() { # key value -> replace in place, append when absent, atomically
  local key=$1 value=$2 tmp
  tmp=$(mktemp "$ENV_FILE.XXXXXX")
  chmod 600 "$tmp"
  awk -v k="$key" -v v="$value" '
    $0 ~ "^[[:space:]]*" k "[[:space:]]*=" { print k "=" v; seen = 1; next }
    { print }
    END { if (!seen) print k "=" v }
  ' "$ENV_FILE" >"$tmp"
  mv "$tmp" "$ENV_FILE"
}

valid_port() {
  case $1 in ''|*[!0-9]*) return 1 ;; esac
  [ "$1" -ge 1 ] && [ "$1" -le 65535 ]
}

docker_ready() { command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; }

public_url() {
  local override scheme host port external
  override=$(read_env UK_PUBLIC_URL)
  if [ -n "$override" ]; then printf '%s\n' "${override%/}"; return 0; fi
  scheme=$(read_env UK_SCHEME); scheme=${scheme:-http}
  host=$(read_env UK_HOST)
  if [ -z "$host" ]; then
    host=$(curl -4 -fsS --max-time 5 https://ifconfig.me 2>/dev/null || true)
  fi
  port=$(read_env UK_PORT); port=${port:-3001}
  external=$(read_env UK_EXTERNAL_PORT)
  [ -n "$external" ] && port=$external
  [ -n "$host" ] || die "cannot detect a public address; set UK_HOST or UK_PUBLIC_URL"
  if [ "$scheme" = https ] && { [ "$port" = 443 ] || [ "$port" = 80 ]; }; then
    printf '%s://%s\n' "$scheme" "$host"
    return 0
  fi
  printf '%s://%s:%s\n' "$scheme" "$host" "$port"
}

apply() {
  if docker_ready; then
    "${COMPOSE[@]}" up -d
    say "URL: $(public_url)"
    return 0
  fi
  say "Docker is not reachable here. On the host run:"
  say "  docker compose -f docker-compose.yml --env-file .env up -d"
  say "Then check: $(public_url 2>/dev/null || echo '<UK_HOST unset>')"
}

command=${1:-help}
case $command in
  up)
    require_env
    docker_ready || die "Docker is not reachable; run this on the VPS"
    "${COMPOSE[@]}" up -d
    say "URL: $(public_url)"
    ;;

  down)
    require_env
    "${COMPOSE[@]}" down
    ;;

  restart)
    require_env
    "${COMPOSE[@]}" restart
    say "URL: $(public_url)"
    ;;

  port)
    require_env
    port=${2:-}
    valid_port "$port" || die "usage: ./uk.sh port <1-65535>"
    set_env UK_PORT "$port"
    say "UK_PORT=$port"
    apply
    ;;

  external-port)
    require_env
    port=${2:-}
    valid_port "$port" || die "usage: ./uk.sh external-port <1-65535>"
    set_env UK_EXTERNAL_PORT "$port"
    say "URL: $(public_url)"
    ;;

  host)
    require_env
    host=${2:-}
    [ -n "$host" ] || die "usage: ./uk.sh host <public-host-or-ip>"
    case $host in *[!A-Za-z0-9.:-]*) die "invalid host: $host" ;; esac
    set_env UK_HOST "$host"
    say "URL: $(public_url)"
    ;;

  url)
    require_env
    public_url
    ;;

  status)
    require_env
    if docker_ready; then
      "${COMPOSE[@]}" ps
      port=$(read_env UK_PORT); port=${port:-3001}
      curl -fsS --max-time 5 -o /dev/null "http://127.0.0.1:$port" \
        && say "local: http://127.0.0.1:$port responds" \
        || say "local: http://127.0.0.1:$port does not respond"
    else
      say "Docker is not reachable; run this on the VPS"
    fi
    say "URL: $(public_url 2>/dev/null || echo '<run ./uk.sh host or port first>')"
    ;;

  logs)
    require_env
    "${COMPOSE[@]}" logs --tail=100 -f "${2:-}"
    ;;

  dgai)
    # Prints the console content JSON for Admin → Content → Uptime Kuma.
    # The dgai console embeds each group as <url>/status/<slug>.
    require_env
    slug=${2:-}
    [ -n "$slug" ] || die "usage: ./uk.sh dgai <status-page-slug>"
    case $slug in *[!A-Za-z0-9_-]*) die "invalid slug: $slug" ;; esac
    printf '[{"id":1,"categoryName":"dgai","url":"%s","slug":"%s"}]\n' \
      "$(public_url)" "$slug"
    ;;

  help|*)
    say "usage: ./uk.sh <command>"
    say ""
    say "  up                 start the stack (needs Docker here)"
    say "  down               stop the stack"
    say "  restart            restart the container"
    say "  port <n>           change the published port and recreate"
    say "  external-port <n>  set the provider-published port used in the URL"
    say "  host <name|ip>     set the public host used in the printed URL"
    say "  url                print the current public URL"
    say "  status             container state and local health"
    say "  logs [service]     follow logs"
    say "  dgai <slug>        print console_setting.uptime_kuma_groups JSON"
    ;;
esac
