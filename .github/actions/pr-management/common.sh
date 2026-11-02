#!/usr/bin/env bash
export TZ=UTC
# Central mutation guard, also used by the offline integration tests.
gh() {
  if [[ "$DRY_RUN" == "true" && "$1" == "pr" && "$2" =~ ^(edit|comment|close)$ ]]; then
    printf '[dry-run]'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  if [[ "$1" == "api" && "$2" == "graphql" ]]; then
    local response="gh-response-${BASHPID:-$$}-${RANDOM}.json"
    if ! command gh "$@" > "$response"; then rm -f -- "$response"; return 1; fi
    if ! jq -e '(.errors == null or (.errors | length == 0)) and
      (.data | type == "object")' "$response" > /dev/null; then
      echo "::error::Incomplete GraphQL response" >&2
      rm -f -- "$response"
      return 1
    fi
    cat "$response"
    rm -- "$response"
    return 0
  fi
  command gh "$@"
}

validate_connection() {
  jq -e "$2 | type == \"object\" and (.nodes | type == \"array\") and
    (.pageInfo | type == \"object\") and (.pageInfo.hasNextPage | type == \"boolean\")" "$1" > /dev/null
}
