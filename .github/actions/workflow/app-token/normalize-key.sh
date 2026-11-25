#!/usr/bin/env bash
# Normalizes the GitHub App private key in RAW_APP_PRIVATE_KEY, which may be
# saved as PEM, as PEM with escaped newlines, or as base64-encoded PEM. Masks
# each line of the key and writes it to GITHUB_OUTPUT as private_key, along
# with the repository name as repository.
set -euo pipefail

pem='BEGIN ([A-Z]+ )?PRIVATE KEY'
key="$RAW_APP_PRIVATE_KEY"
# Support secrets saved with escaped newlines.
key="${key//\\n/$'\n'}"

if ! printf '%s' "$key" | grep -Eq "$pem"; then
  # Support secrets saved as base64-encoded PEM.
  decoded="$(printf '%s' "$RAW_APP_PRIVATE_KEY" | tr -d '[:space:]' | base64 --decode 2>/dev/null || true)"
  decoded="${decoded//\\n/$'\n'}"
  if printf '%s' "$decoded" | grep -Eq "$pem"; then
    key="$decoded"
  fi
fi

if ! printf '%s' "$key" | grep -Eq "$pem"; then
  echo "The GitHub App private key does not look like a valid PEM private key." >&2
  exit 1
fi

# ::add-mask:: is line-based, mask each line of the key individually.
while IFS= read -r line; do
  [ -n "$line" ] && echo "::add-mask::$line"
done <<< "$key"
{
  echo "private_key<<EOF"
  printf '%s\n' "$key"
  echo "EOF"
  # github.event.repository is missing from schedule events.
  echo "repository=${GITHUB_REPOSITORY#*/}"
} >> "$GITHUB_OUTPUT"
