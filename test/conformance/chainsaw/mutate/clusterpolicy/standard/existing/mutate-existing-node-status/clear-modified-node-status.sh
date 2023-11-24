#!/usr/bin/env bash
set -eu

kubectl proxy &
proxy_pid=$!
echo $proxy_pid

function cleanup {
  echo "killing kubectl proxy" >&2
  kill $proxy_pid
}

attempt_counter=0
max_attempts=5

until curl --output /dev/null -fsSL http://localhost:8001/; do
  if [ ${attempt_counter} -eq ${max_attempts} ]; then
    echo "Max attempts reached"
    exit 1
  fi

  attempt_counter=$((attempt_counter + 1))
  sleep 5
done

curl --header "Content-Type: application/json-patch+json" \
  --request PATCH \
  --output /dev/null \
  --data '[{"op": "remove", "path": "/status/capacity/example.com~1dongle"}]' \
  http://localhost:8001/api/v1/nodes/kind-control-plane/status

kubectl annotate node kind-control-plane policies.kyverno.io/last-applied-patches-
trap cleanup EXIT