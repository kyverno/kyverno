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

if [ "$service" == "" ]; then
    service="kyverno-svc"
fi

destdir="certs"
if [ ! -d "$destdir" ]; then
  mkdir ${destdir} || exit 1
fi

tmpdir=$(mktemp -d)
cat <<EOF >> ${tmpdir}/csr.conf
[req]
req_extensions      = v3_req
distinguished_name  = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints    = CA:FALSE
keyUsage            = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage    = serverAuth
subjectAltName      = @alt_names
[alt_names]
DNS.1               = ${service}
IP.1                = ${serverIP}
EOF

if [ ! -z "${service}" ]; then
    subjectCN="${service}"
else
  subjectCN=${serverIP}
fi

echo "Generating self-signed certificate for CN=${subjectCN}"
# generate priv key for root CA
openssl genrsa -out ${destdir}/rootCA.key 4096 
# generate root CA
openssl req -x509 -new -nodes -key ${destdir}/rootCA.key -sha256 -days 1024 -out ${destdir}/rootCA.crt -subj "/CN=${subjectCN}"
# generate priv key
openssl genrsa -out ${destdir}/webhook.key 4096
# generate certificate
openssl req -new -key ${destdir}/webhook.key -out ${destdir}/webhook.csr -subj "/CN=${subjectCN}" -config ${tmpdir}/csr.conf
# sign the certificate using the root CA
openssl x509 -req -in ${destdir}/webhook.csr -CA ${destdir}/rootCA.crt -CAkey ${destdir}/rootCA.key -CAcreateserial -out ${destdir}/webhook.crt -days 1024 -sha256 -extensions v3_req  -extfile ${tmpdir}/csr.conf


kubectl delete -f config/install_debug.yaml 2>/dev/null
kubectl delete namespace kyverno 2>/dev/null

echo "Generating corresponding kubernetes secrets for TLS pair and root CA"
# create project namespace
kubectl create ns kyverno
# create tls pair secret
kubectl -n kyverno create secret tls ${service}.kyverno.svc.kyverno-tls-pair --cert=${destdir}/webhook.crt --key=${destdir}/webhook.key
# annotate tls pair secret to specify use of self-signed certificates and check if root CA is created as secret
kubectl annotate secret ${service}.kyverno.svc.kyverno-tls-pair -n kyverno self-signed-cert=true
# create root CA secret
kubectl -n kyverno create secret generic ${service}.kyverno.svc.kyverno-tls-ca --from-file=${destdir}/rootCA.crt

echo "Creating CRD"
kubectl apply -f config/install_debug.yaml
