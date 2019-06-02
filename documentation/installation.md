<small>*[documentation](/README.md#documentation) / Installation*</small>

# Installation

The Kyverno policy engine runs as an admission webhook and requires a CA-signed certificate and key to setup secure TLS communication with the kube-apiserver (the CA can be self-signed). 

There are 2 ways to configure the secure communications link between Kyverno and the kube-apiserver:

**Option 1: Use your own CA-signed certificate**

You can install your own CA-signed certificate, or generate a self-signed CA and use it to sign a certifcate. Once you have a CA and X.509 certificate-key pair, you can install these as Kubernetes secrets in your cluster. If Kyverno finds these secrets, it uses them. Otherwise it will request the `kube-controller-manager` to generate a certificate (see Option 2 below).

**1. Generate a self-signed CA and signed certificate-key pair**

**Note: using a separate self-signed root CA is difficult to manage and not recommeded for production use.** 

If you already have a CA and a signed certificate, you can directly proceed to Step 2.

Here are the commands to create a self-signed root CA, and generate a signed certificate and key using openssl (you can customize the certificate attributes for your deployment):

````bash
openssl genrsa -out rootCA.key 4096
openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 1024 -out rootCA.crt  -subj \"/C=US/ST=test/L=test /O=test /OU=PIB/CN=*.kyverno.svc/emailAddress=test@test.com\"
openssl genrsa -out webhook.key 4096
openssl req -new -key webhook.key -out webhook.csr  -subj \"/C=US/ST=test /L=test /O=test /OU=PIB/CN=kyverno-svc.kyverno.svc/emailAddress=test@test.com\"
openssl x509 -req -in webhook.csr -CA rootCA.crt -CAkey rootCA.key -CAcreateserial -out webhook.crt -days 1024 -sha256
````

The following files will be generated and can be used to create Kubernetes secrets:
- rootCA.crt
- webhooks.crt
- webhooks.key

**2. Configure secrets for the CA and TLS certificate-key pair**

To create the required secrets, use the following commands (do not change the secret names):

````bash
kubectl create ns kyverno
kubectl -n kyverno create secret tls kyverno-svc.kyverno.svc.kyverno-tls-pair --cert=webhook.crt --key=webhook.key
kubectl annotate secret kyverno-svc.kyverno.svc.kyverno-tls-pair -n kyverno self-signed-cert=true
kubectl -n kyverno create secret generic kyverno-svc.kyverno.svc.kyverno-tls-ca --from-file=rootCA.crt
````

**NOTE: The annotation on the TLS pair secret is used by Kyverno to identify the use of self-signed certificates and checks for the required root CA secret**

Secret | Data | Content
------------ | ------------- | -------------
`kyverno-svc.kyverno.svc.kyverno-tls-pair` | rootCA.crt | root CA used to sign the certificate
`kyverno-svc.kyverno.svc.kyverno-tls-ca` | tls.key & tls.crt  | key and signed certificate

Kyverno uses secrets created above to setup TLS communication with the kube-apiserver and specify the CA bundle to be used to validate the webhook server's certificate in the admission webhook configurations.

**3. Install Kyverno**

````sh
kubectl create -f https://github.com/nirmata/kyverno/raw/master/definitions/install.yaml
````

To check the Kyverno controller status, run the command:

````sh
kubectl get pods -n kyverno
````

If the Kyverno controller is not running, you can check its status and logs for errors:

````sh
kubectl describe pod <kyverno-pod-name> -n kyverno
````

````sh
kubectl logs <kyverno-pod-name> -n kyverno
````

Here is a script that automates these steps. generates a self-signed CA, a TLS certificate-key pair, and the corresponding kubernetes secrets: [helper script](/scripts/generate-self-signed-cert-and-k8secrets.sh)


**Option 2: Use `kube-controller-manager` to generate a CA-signed certificate**

Kyverno can request a CA signed certificate-key pair from `kube-controller-manager`. This method requires that the kube-controller-manager is configured to act as a certificate signer. To verify that this option is enabled for your cluster, check the command-line args for the kube-controller-manager. If `--cluster-signing-cert-file` and `--cluster-signing-key-file` are passed to the controller manager with paths to your CA's key-pair, then you can proceed to install Kyverno using this method.

To install Kyverno in a cluster that supports certificate signing, run the following command on a host with kubectl `cluster-admin` access:

````sh
kubectl create -f https://github.com/nirmata/kyverno/raw/master/definitions/install.yaml
````

To check the Kyverno controller status, run the command:

````sh
kubectl get pods -n kyverno
````

If the Kyverno controller is not running, you can check its status and logs for errors:

````sh
kubectl describe pod <kyverno-pod-name> -n kyverno
````

````sh
kubectl logs <kyverno-pod-name> -n kyverno
````


# Installing in a Development Environment

To build and run Kyverno in a development environment see: https://github.com/nirmata/kyverno/wiki/Building

To check if the controller is working, find it in the list of kyverno pods:

`kubectl get pods -n kyverno`

# Try Kyverno without a Kubernetes cluster

The [Kyverno CLI](documentation/testing-policies-cli.md) allows you to write and test policies without installing Kyverno in a Kubernetes cluster. Some features are not supported without a Kubernetes cluster.



---
<small>*Read Next >> [Writing Policies](/documentation/writing-policies.md)*</small>
