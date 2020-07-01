<small>*[documentation](/README.md#documentation) / Installation*</small>

# Installation

You can install Kyverno using the Helm chart or YAML files in this repository.

## Install Kyverno using Helm

```sh

## Add the nirmata Helm repository
helm repo add kyverno https://nirmata.github.io/kyverno/

## Create the Kyverno namespace
kubectl create ns kyverno

## Install the kyverno helm chart
helm install kyverno --namespace kyverno kyverno/kyverno

```

## Install Kyverno using YAMLs

The Kyverno policy engine runs as an admission webhook and requires a CA-signed certificate and key to setup secure TLS communication with the kube-apiserver (the CA can be self-signed). 

There are 2 ways to configure the secure communications link between Kyverno and the kube-apiserver.

### Option 1: Use kube-controller-manager to generate a CA-signed certificate

Kyverno can request a CA signed certificate-key pair from `kube-controller-manager`. This method requires that the kube-controller-manager is configured to act as a certificate signer. To verify that this option is enabled for your cluster, check the command-line args for the kube-controller-manager. If `--cluster-signing-cert-file` and `--cluster-signing-key-file` are passed to the controller manager with paths to your CA's key-pair, then you can proceed to install Kyverno using this method.

**Deploying on EKS requires enabling a command-line argument `--fqdn-as-cn` in the 'kyverno' container in the deployment, due to a current limitation with the certificates returned by EKS for CSR(bug: https://github.com/awslabs/amazon-eks-ami/issues/341)**

To install Kyverno in a cluster that supports certificate signing, run the following command on a host with kubectl `cluster-admin` access:

Note that the above command will install the last released (stable) version of Kyverno. If you want to install the latest version, you can edit the [install.yaml] and update the image tag. 

To check the Kyverno controller status, run the command:

```sh
## Install Kyverno
kubectl create -f https://github.com/nirmata/kyverno/raw/master/definitions/install.yaml

## Check pod status
kubectl get pods -n kyverno
````

If the Kyverno controller is not running, you can check its status and logs for errors:

````sh
kubectl describe pod <kyverno-pod-name> -n kyverno
````

````sh
kubectl logs <kyverno-pod-name> -n kyverno
````

### Option 2: Use your own CA-signed certificate

You can install your own CA-signed certificate, or generate a self-signed CA and use it to sign a certifcate. Once you have a CA and X.509 certificate-key pair, you can install these as Kubernetes secrets in your cluster. If Kyverno finds these secrets, it uses them. Otherwise it will request the kube-controller-manager to generate a certificate (see Option 1 above).

#### 1. Generate a self-signed CA and signed certificate-key pair

**Note: using a separate self-signed root CA is difficult to manage and not recommeded for production use.** 

If you already have a CA and a signed certificate, you can directly proceed to Step 2.

Here are the commands to create a self-signed root CA, and generate a signed certificate and key using openssl (you can customize the certificate attributes for your deployment):

````bash
openssl genrsa -out rootCA.key 4096
openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 1024 -out rootCA.crt  -subj "/C=US/ST=test/L=test /O=test /OU=PIB/CN=*.kyverno.svc/emailAddress=test@test.com"
openssl genrsa -out webhook.key 4096
openssl req -new -key webhook.key -out webhook.csr  -subj "/C=US/ST=test /L=test /O=test /OU=PIB/CN=kyverno-svc.kyverno.svc/emailAddress=test@test.com"
openssl x509 -req -in webhook.csr -CA rootCA.crt -CAkey rootCA.key -CAcreateserial -out webhook.crt -days 1024 -sha256
````

Among the files that will be generated, you can use the following files to create Kubernetes secrets:
- rootCA.crt
- webhooks.crt
- webhooks.key

#### 2. Configure secrets for the CA and TLS certificate-key pair

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

#### 3. Configure Kyverno Role
Kyverno, in `foreground` mode, leverages admission webhooks to manage incoming api-requests, and `background` mode applies the policies on existing resources. It uses ServiceAccount `kyverno-service-account`, which is bound to multiple ClusterRole, which defines the default resources and operations that are permitted.

ClusterRoles used by kyverno:
- kyverno:webhook
- kyverno:userinfo
- kyverno:customresources
- kyverno:policycontroller
- kyverno:generatecontroller

The `generate` rule creates a new resource, and to allow kyverno to create resource kyverno ClusterRole needs permissions to create/update/delete. This can be done by adding the resource to the ClusterRole `kyverno:generatecontroller` used by kyverno or by creating a new ClusterRole and a ClusterRoleBinding to kyverno's default ServiceAccount.

```yaml
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: kyverno:generatecontroller
rules:
- apiGroups:
  - "*"
  resources:
  - namespaces
  - networkpolicies
  - secrets
  - configmaps
  - resourcequotas
  - limitranges
  - ResourceA # new Resource to be generated
  - ResourceB
  verbs:
  - create # generate new resources
  - get # check the contents of exiting resources
  - update # update existing resource, if required configuration defined in policy is not present
  - delete # clean-up, if the generate trigger resource is deleted
```
```yaml
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: kyverno-admin-generate
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kyverno:generatecontroller # clusterRole defined above, to manage generated resources
subjects:
- kind: ServiceAccount
  name: kyverno-service-account # default kyverno serviceAccount
  namespace: kyverno
```

#### 4. Install Kyverno

To install a specific version, download [install.yaml] and then change the image tag.

e.g., change image tag from `latest` to the specific tag `v1.0.0`.
>>>
    spec:
      containers:
        - name: kyverno
          # image: nirmata/kyverno:latest
          image: nirmata/kyverno:v1.0.0
          
````sh
kubectl create -f ./install.yaml
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

Here is a script that generates a self-signed CA, a TLS certificate-key pair, and the corresponding kubernetes secrets: [helper script](/scripts/generate-self-signed-cert-and-k8secrets.sh)



# Configure a namespace admin to access policy violations

During Kyverno installation, it creates a ClusterRole `kyverno:policyviolations` which has the `list,get,watch` operations on resource `policyviolations`. To grant access to a namespace admin, configure the following YAML file then apply to the cluster.

- Replace `metadata.namespace` with namespace of the admin
- Configure `subjects` field to bind admin's role to the ClusterRole `policyviolation`

````yaml
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: policyviolation
  # change namespace below to create rolebinding for the namespace admin
  namespace: default
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: policyviolation
subjects:
# configure below to access policy violation for the namespace admin
- kind: ServiceAccount
  name: default
  namespace: default
# - apiGroup: rbac.authorization.k8s.io
#   kind: User
#   name: 
# - apiGroup: rbac.authorization.k8s.io
#   kind: Group
#   name: 
````
# Installing outside of the cluster (debug mode)

To build Kyverno in a development environment see: https://github.com/nirmata/kyverno/wiki/Building

To run controller in this mode you should prepare a TLS key/certificate pair for debug webhook, then start controller with kubeconfig and the server address.

1. Run `sudo scripts/deploy-controller-debug.sh --service=localhost --serverIP=<server_IP>`, where <server_IP> is the IP address of the host where controller runs. This scripts will generate a TLS certificate for debug webhook server and register this webhook in the cluster. It also registers a CustomResource policy.

2. Start the controller using the following command: `sudo KYVERNO_NAMESPACE=kyverno go run ./cmd/kyverno/main.go --kubeconfig=~/.kube/config --serverIP=<server_IP>`


# Filter Kubernetes resources that admission webhook should not process
The admission webhook checks if a policy is applicable on all admission requests. The Kubernetes kinds that are not be processed can be filtered by adding a `ConfigMap` in namespace `kyverno` and specifying the resources to be filtered under `data.resourceFilters`. The default name of this `ConfigMap` is `init-config` but can be changed by modifying the value of the environment variable `INIT_CONFIG` in the kyverno deployment dpec. `data.resourceFilters` must be a sequence of one or more `[<Kind>,<Namespace>,<Name>]` entries with `*` as wildcard. Thus, an item `[Node,*,*]` means that admissions of `Node` in any namespace and with any name will be ignored.

By default we have specified Nodes, Events, APIService & SubjectAccessReview as the kinds to be skipped in the default configuration [install.yaml].

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: init-config
  namespace: kyverno
data:
  # resource types to be skipped by kyverno policy engine
  resourceFilters: "[Event,*,*][*,kube-system,*][*,kube-public,*][*,kube-node-lease,*][Node,*,*][APIService,*,*][TokenReview,*,*][SubjectAccessReview,*,*][*,kyverno,*]"
```

To modify the `ConfigMap`, either directly edit the `ConfigMap` `init-config` in the default configuration [install.yaml] and redeploy it or modify the `ConfigMap` use `kubectl`.  Changes to the `ConfigMap` through `kubectl` will automatically be picked up at runtime.


---
<small>*Read Next >> [Writing Policies](/documentation/writing-policies.md)*</small>

[install.yaml]: https://github.com/nirmata/kyverno/raw/master/definitions/install.yaml
