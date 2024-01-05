#!/bin/bash
set -euo pipefail

export USERNAME=chip
export NAMESPACE=qa
export CA=ca.crt
####
#### Get CA certificate from kubeconfig assuming it's the first in the list.
kubectl config view --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}' | base64 --decode > ca.crt
#### Set CLUSTER_SERVER from kubeconfig assuming it's the first in the list.
CLUSTER_SERVER=$(kubectl config view --raw -o jsonpath='{.clusters[0].cluster.server}')
#### Set CLUSTER from kubeconfig assuming it's the first in the list.
CLUSTER=$(kubectl config view --raw -o jsonpath='{.clusters[0].name}')
#### Generate private key
openssl genrsa -out $USERNAME.key 2048
#### Create CSR
openssl req -new -key $USERNAME.key -out $USERNAME.csr -subj "/O=mygroup/CN=$USERNAME"
#### Send CSR to kube-apiserver for approval
cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: $USERNAME
spec:
  request: $(cat $USERNAME.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF
#### Approve CSR
kubectl certificate approve $USERNAME
#### Download certificate
kubectl get csr $USERNAME -o jsonpath='{.status.certificate}' | base64 --decode > $USERNAME.crt
####
#### Create the credential object and output the new kubeconfig file
kubectl --kubeconfig=$USERNAME-kubeconfig config set-credentials $USERNAME --client-certificate=$USERNAME.crt --client-key=$USERNAME.key --embed-certs
#### Set the cluster info
kubectl --kubeconfig=$USERNAME-kubeconfig config set-cluster $CLUSTER --server=$CLUSTER_SERVER --certificate-authority=$CA --embed-certs
#### Set the context
kubectl --kubeconfig=$USERNAME-kubeconfig config set-context $USERNAME-$NAMESPACE-$CLUSTER --user=$USERNAME --cluster=$CLUSTER --namespace=$NAMESPACE
#### Use the context
kubectl --kubeconfig=$USERNAME-kubeconfig config use-context $USERNAME-$NAMESPACE-$CLUSTER
### Clean up the approved CSR
kubectl delete certificatesigningrequest chip