#!/bin/bash
hub_user_name="nirmata"
project_name="kube-policy"
echo ${1}
namespace=${1}
if [ ${namespace} -eq "" ]; then
  echo "Specify target namespace in the first parameter"
  exit 1
fi

service_name="${project_name}-svc"
echo "Generating certificate for the service ${service_name}..."
serverIp="192.168.10.177" #TODO: ! Read it from ~/.kube/config !
certsGenerator="./scripts/generate-server-cert.sh"
chmod +x "${certsGenerator}"
${certsGenerator} ${service_name} ${namespace} ${serverIp} || exit 2

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
