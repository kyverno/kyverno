#!/bin/bash

for i in "$@"
do
case $i in
    --namespace=*)
    namespace="${i#*=}"
    shift
    ;;
    --serverIp=*)
    serverIp="${i#*=}"
    shift
    ;;
esac
done

hub_user_name="nirmata"
project_name="kube-policy"

service_name="${project_name}-svc"
echo "Generating certificate for the service ${service_name}..."

certsGenerator="./scripts/generate-server-cert.sh"
chmod +x "${certsGenerator}"

if [ -z "${namespace}" ]; then # controller is launched locally

  if [ -z "${serverIp}" ]; then
    echo "--serverIp should be explicitly specified if --namespace is empty"
    exit 1
  fi

  ${certsGenerator} "--serverIp=${serverIp}" || exit 2

  echo "Applying webhook..."
  kubectl delete -f crd/MutatingWebhookConfiguration_local.yaml
  kubectl create -f crd/MutatingWebhookConfiguration_local.yaml || exit 3

  echo -e "\n### You can build and run kube-policy project locally.\n### To check its work, run it with parameters -cert and -key, which contain generated TLS certificate and key (see their paths in log above)."

else # controller is launched within a cluster

  ${certsGenerator} "--service=${service_name}" "--namespace=${namespace}" "--serverIp=${serverIp}" || exit 2

  secret_name="${project_name}-secret"
  echo "Generating secret ${secret_name}..."
  kubectl delete secret "${secret_name}" 2>/dev/null
  kubectl create secret generic ${secret_name} --namespace ${namespace} --from-file=./certs || exit 3

  echo "Creating the service ${service_name}..."
  kubectl delete -f crd/service.yaml
  kubectl create -f crd/service.yaml || exit 4

  echo "Creating deployment..."
  kubectl delete -f crd/deployment.yaml
  kubectl create -f crd/deployment.yaml || exit 5

  echo "Applying webhook..."
  kubectl delete -f crd/MutatingWebhookConfiguration.yaml
  kubectl create -f crd/MutatingWebhookConfiguration.yaml || exit 3

  echo -e "\n### Controller is running in cluster.\n### You can use compile-image.sh to rebuild its image and then the current script to redeploy the controller.\n### Check its work by 'kubectl logs <controller_pod> command'"

fi
