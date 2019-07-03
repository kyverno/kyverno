#!/bin/bash

for i in "$@"
do
case $i in
    --service=*)
    service="${i#*=}"
    shift
    ;;
    --serverIP=*)
    serverIP="${i#*=}"
    shift
    ;;
esac
done

if [ -z "${serverIP}" ]; then
  echo -e "Please specify '--serverIP' where Kyverno controller runs."
  exit 1
fi

if [ -z "${service}" ]; then
  service="localhost"
fi

echo "service is $service"
echo "serverIP is $serverIP"

echo "Generating certificate for the service ${service}..."

certsGenerator="./scripts/generate-self-signed-cert-and-k8secrets-debug.sh"
chmod +x "${certsGenerator}"

${certsGenerator} "--service=${service}" "--serverIP=${serverIP}" || exit 2
echo -e "\n### You can build and run kyverno project locally.\n### To check its work, run it with flags --kubeconfig and --serverIP parameters."
