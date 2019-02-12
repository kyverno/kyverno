#!/bin/bash
service=${1}
namespace=${2}
serverIp=${3}

destdir="certs"
if [ ! -d "$destdir" ]; then
  mkdir ${destdir}
fi
tmpdir=$(mktemp -d)

cat <<EOF >> ${tmpdir}/csr.conf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${service}
DNS.2 = ${service}.${namespace}
DNS.3 = ${service}.${namespace}.svc
DNS.4 = ${serverIp}
EOF

outKeyFile=${destdir}/server-key.pem
outCertFile=${destdir}/server.crt

openssl genrsa -out ${outKeyFile} 2048
openssl req -new -key ${destdir}/server-key.pem -subj "/CN=${service}.${namespace}.svc" -out ${tmpdir}/server.csr -config ${tmpdir}/csr.conf

CSR_NAME=${service}.cert-request
kubectl delete csr ${CSR_NAME} 2>/dev/null

cat <<EOF | kubectl create -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${CSR_NAME}
spec:
  groups:
  - system:authenticated
  request: $(cat ${tmpdir}/server.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF

kubectl certificate approve ${CSR_NAME}
kubectl get csr ${CSR_NAME} -o jsonpath='{.status.certificate}' | base64 --decode > ${outCertFile}

echo "Generated:"
echo ${outKeyFile}
echo ${outCertFile}
