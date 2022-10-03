#!/bin/bash


for i in "$@"
do
case $i in
    --service=*)
    service="${i#*=}"
    shift
    ;;
    --namespace=*)
    namespace="${i#*=}"
    shift
    ;;
esac
done

if [ "$service" == "" ]; then
    service="kyverno-svc"
fi


if [ "$namespace" == "" ]; then
    namespace="kyverno"
fi

echo "service is $service"
echo "namespace is $namespace"

echo "Generating self-signed certificate"
# generate priv key for root CA
openssl genrsa -out rootCA.key 4096
# generate root CA
openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 1024 -out rootCA.crt -subj "/C=US/ST=test/L=test /O=test /OU=PIB/CN=*.${namespace}.svc/emailAddress=test@test.com"
# generate priv key
openssl genrsa -out webhook.key 4096
# generate certificate
openssl req -new -key webhook.key -out webhook.csr -subj "/C=US/ST=test /L=test /O=test /OU=PIB/CN=${service}.${namespace}.svc/emailAddress=test@test.com"

# generate SANs
echo "subjectAltName = DNS:kyverno-svc,DNS:${service}.${namespace},DNS:${service}.${namespace}.svc" >> webhook.ext

# sign the certificate using the root CA
openssl x509 -req -in webhook.csr -CA rootCA.crt -CAkey rootCA.key -CAcreateserial -out webhook.crt -days 1024 -sha256

echo "Generating corresponding kubernetes secrets for TLS pair and root CA"
# create project namespace
kubectl create ns ${namespace}
# create tls pair secret
kubectl -n ${namespace} create secret tls ${service}.${namespace}.svc.kyverno-tls-pair --cert=webhook.crt --key=webhook.key
# annotate tls pair secret to specify use of self-signed certificates and check if root CA is created as secret
kubectl annotate secret ${service}.${namespace}.svc.kyverno-tls-pair -n ${namespace} self-signed-cert=true
# create root CA secret
kubectl -n ${namespace} create secret generic ${service}.${namespace}.svc.kyverno-tls-ca --from-file=rootCA.crt
