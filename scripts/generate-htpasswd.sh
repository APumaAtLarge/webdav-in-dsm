#!/bin/sh
set -eu

HTPASSWD_FILE="${HTPASSWD_FILE:-.htpasswd}"
BASIC_AUTH_USER="${BASIC_AUTH_USER:-}"
BASIC_AUTH_PASSWORD="${BASIC_AUTH_PASSWORD:-}"

if [ -z "$BASIC_AUTH_USER" ]; then
  echo "BASIC_AUTH_USER is required" >&2
  exit 1
fi

if [ -z "$BASIC_AUTH_PASSWORD" ]; then
  echo "BASIC_AUTH_PASSWORD is required" >&2
  exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required to generate $HTPASSWD_FILE" >&2
  exit 1
fi

umask 077
HASH="$(printf '%s\n' "$BASIC_AUTH_PASSWORD" | openssl passwd -apr1 -stdin)"
printf '%s:%s\n' "$BASIC_AUTH_USER" "$HASH" > "$HTPASSWD_FILE"
echo "Generated $HTPASSWD_FILE for user $BASIC_AUTH_USER"
