#!/bin/bash

for i in "$@"
do
case $i in
    --service=*)
    service_name="${i#*=}"
    shift
    ;;
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
project_name="kyverno"

if [ -z "${service_name}" ]; then
  service_name="${project_name}-svc"
fi
echo "Generating certificate for the service ${service_name}..."

certsGenerator="./scripts/generate-server-cert.sh"
chmod +x "${certsGenerator}"

if [ -z "${namespace}" ]; then # controller should be launched locally

  ${certsGenerator} "--service=${service_name}" "--serverIp=${serverIp}" || exit 2

  kubectl delete -f config/install.yaml
  kubectl create -f config/install.yaml || exit 3

  echo -e "\n### You can build and run kyverno project locally.\n### To check its work, run it with parameters -cert, -key and -kubeconfig parameters (see paths of -cert and -key in the log above)."

else # controller should be launched within a cluster

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

  kubectl delete -f crd/crd.yaml
  kubectl create -f crd/crd.yaml || exit 3

  echo -e "\n### Controller is running in cluster.\n### You can use compile-image.sh to rebuild its image and then the current script to redeploy the controller.\n### Check its work by 'kubectl logs <controller_pod> command'"

fi
