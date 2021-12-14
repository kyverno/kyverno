#!/bin/bash
# Waits for a deployment to complete.
#
# Includes a two-step approach:
#
# 1. Wait for the observed generation to match the specified one.
# 2. Waits for the number of available replicas to match the specified one.
#

set -o errexit
set -o pipefail
set -o nounset
# -m enables job control which is otherwise only enabled in interactive mode
set -m

DEFAULT_TIMEOUT=60
DEFAULT_NAMESPACE=default

monitor_timeout() {
  local -r wait_pid="$1"
  sleep "${timeout}"
  echo "Timeout ${timeout} exceeded" >&2
  kubectl --namespace "${namespace}" get pods
  docker images | grep "kyverno"
  kubectl --namespace "${namespace}" describe deployment "${deployment}"
  kubectl --namespace "${namespace}" logs -l app=kyverno
  kill "${wait_pid}"
}

get_generation() {
  get_deployment_jsonpath '{.metadata.generation}'
}

get_observed_generation() {
  get_deployment_jsonpath '{.status.observedGeneration}'
}

get_specified_replicas() {
  get_deployment_jsonpath '{.spec.replicas}'
}

get_replicas() {
  get_deployment_jsonpath '{.status.replicas}'
}

get_updated_replicas() {
  get_deployment_jsonpath '{.status.updatedReplicas}'
}

get_available_replicas() {
  get_deployment_jsonpath '{.status.availableReplicas}'
}

get_deployment_jsonpath() {
  local -r jsonpath="$1"

  kubectl --namespace "${namespace}" get deployment "${deployment}" -o "jsonpath=${jsonpath}"
}

display_usage_and_exit() {
  echo "Usage: $(basename "$0") [-n <namespace>] [-t <timeout>] <deployment>" >&2
  echo "Arguments:" >&2
  echo "deployment REQUIRED: The name of the deployment the script should wait on" >&2
  echo "-n OPTIONAL: The namespace the deployment exists in, defaults is the 'default' namespace" >&2
  echo "-t OPTIONAL: How long to wait for the deployment to be available, defaults to ${DEFAULT_TIMEOUT} seconds, must be greater than 0" >&2
  exit 1
}

namespace=${DEFAULT_NAMESPACE}
timeout=${DEFAULT_TIMEOUT}

while getopts ':n:t:' arg
do
    case ${arg} in
        n) namespace=${OPTARG};;
        t) timeout=${OPTARG};;
        *) display_usage_and_exit
    esac
done

shift $((OPTIND-1))
if [ "$#" -ne 1 ] ; then
  display_usage_and_exit
fi
readonly deployment="$1"

if [[ ${timeout} -le 0 ]]; then
  display_usage_and_exit
fi

echo "Waiting for deployment of ${deployment} in namespace ${namespace} with a timeout ${timeout} seconds"

monitor_timeout $$ &
readonly timeout_monitor_pid=$!

trap 'kill -- -${timeout_monitor_pid}' EXIT #Stop timeout monitor

generation=$(get_generation);  readonly generation
current_generation=$(get_observed_generation)

echo "Expected generation for deployment ${deployment}: ${generation}"
while [[ ${current_generation} -lt ${generation} ]]; do
  sleep .5
  echo "Currently observed generation: ${current_generation}"
  current_generation=$(get_observed_generation)
done
echo "Observed expected generation: ${current_generation}"

specified_replicas="$(get_specified_replicas)"; readonly specified_replicas
echo "Specified replicas: ${specified_replicas}"

current_replicas=$(get_replicas)
updated_replicas=$(get_updated_replicas)
available_replicas=$(get_available_replicas)

while [[ ${updated_replicas} -lt ${specified_replicas} || ${current_replicas} -gt ${updated_replicas} || ${available_replicas} -lt ${updated_replicas} ]]; do
  sleep .5
  echo "current/updated/available replicas: ${current_replicas}/${updated_replicas}/${available_replicas}, waiting"
  current_replicas=$(get_replicas)
  updated_replicas=$(get_updated_replicas)
  available_replicas=$(get_available_replicas)
done

echo "Deployment ${deployment} successful. All ${available_replicas} replicas are ready."

mutatingwebhookconfigurations=$(kubectl get mutatingwebhookconfigurations | wc -l)
validatingwebhookconfigurations=$(kubectl get validatingwebhookconfigurations | wc -l)
while [[ ${mutatingwebhookconfigurations} -lt 4 || ${validatingwebhookconfigurations} -lt 3 ]]; do
  sleep 5
done

echo "All webhooks are registered."