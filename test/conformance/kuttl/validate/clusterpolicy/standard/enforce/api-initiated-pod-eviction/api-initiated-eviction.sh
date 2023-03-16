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
    if [ ${attempt_counter} -eq ${max_attempts} ];then
      echo "Max attempts reached"
      exit 1
    fi

    attempt_counter=$((attempt_counter+1))
    sleep 5
done

if curl -v -H 'Content-type: application/json' \
  http://localhost:8001/api/v1/namespaces/test-validate/pods/nginx/eviction -d @eviction.json 2>&1 | grep -q "Evicting Pods protected with the label 'evict=false' is forbidden"; then
  echo "Test succeeded. Resource was not evicted."
  trap cleanup EXIT
  exit 0
else
  echo "Tested failed. Resource was evicted."
  trap cleanup EXIT
  exit 1
fi
