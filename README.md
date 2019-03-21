# kube-policy
A Kubernetes native policy engine

## Motivation

## How it works
The solution provides a possibility to validate the custom Kubernetes resources and modify them before their creation. 
### Components

* **Policy Controller** (`/controller`) allows defining custom resources which can be used in your Kubernetes cluster
* **WebHooks Server** (`/server`) implements connection between Kubernetes API server and **Mutation WebHook**
* **Mutation WebHook** (`/webhooks`) allows applying Nirmata policies for validation and mutation of the certain types of resources (see the list below)
* **Kube Client** (`/kubeclient`) allows other components to communicate with Kubernetes API server for resource management in a cluster
* **Initialization functions** (`/init.go`, `/utils`) allow running the controller inside the cluster without deep pre-tuning

The program initializes the configuration of the client API Kubernetes and creates an HTTPS server with a webhook for resource mutation. When a resource is created in a cluster for various reasons, the Kerbernetes core sends a request for a mutation of this resource to the webhook. The policy controller manages the objects of the policies created in the cluster and is always aware of which policies are currently in effect: information on the policies is available on the webhook due to the policy controller. The request to create a resource contains its full definition. If the resource matches to one or more of the current policies, the resource is mutated in accordance with them.

### Policy application

**Supported resource types:**
* ConfigMap
* CronJob
* DaemonSet
* Deployment
* Endpoints
* HorizontalPodAutoscaler
* Ingress
* Job
* LimitRange
* Namespace
* NetworkPolicy
* PersistentVolumeClaim
* PodDisruptionBudget
* PodTemplate
* ResourceQuota
* Secret
* Service
* StatefulSet

When a request for a resource creation is received (i.e. a YAML file), it will be checked against the corresponding Nirmata policies. 
The policy for a resource is looked up either by the resource name, or with the help of selector. 
In case the data in the YAML file does not conform to the policy, the resource will be mutated with the help of the **Mutation WebHook**, which can perform one of the following:

* **add**: either add a lacking key and its value or replace a value of the already existing key;
* **replace**: either replace a value of the already existing key or add a lacking key and its value;
* **remove**: remove an unnecessary key and its value. 

**NOTE**: **add** and **replace** behave in the same way, so they can be used interchangeably. However, there is the difference between the **add** and **replace** operations when mutating an array. In this case **add** will add an element to the list, whereas **replace** will replace the whole list.

After the resource YAML file is mutated, the required object is created in the Kubernetes cluster. 

## Examples

### 1. Mutation of deployment resource
Here is the policy:
```
apiVersion : policy.nirmata.io/v1alpha1
kind : Policy
metadata : 
  name : policy-deployment-ghost   
spec :
  failurePolicy: stopOnError
  rules:
    - resource:
        kind : Deployment    
        name :
        selector :
          matchLabels :
           nirmata.io/deployment.name: "ghost"
      patch:
      - path: /metadata/labels/isMutated
        op: add
        value: "true"
      - path: "/spec/strategy/rollingUpdate/maxSurge"
        op: add
        value: 5
      - path: "/spec/template/spec/containers/0/ports/0"
        op: replace
        value:
          containerPort: 2368
          protocol: TCP
  ```
 
In the **name** parameter, you should specify the policy name.

The **failurePolicy** parameter is optional. It is set to **stopOnError** by default. Other possible value is **continueOnError**. If **continueOnError** is specified, the resource will be created despite the errors that may have occurred in the webhook.

The **rules** section consists of the mandatory **resource** sub-section and an optional **patch** sub-section.

The **resource** sub-section defines to which kind of the supported resources a Nirmata policy has to be applied:

* In the **kind** parameter, you should specify the resource type. You can find the list of the supported types in the **How it works** section. 
* In the **name** parameter, you should specify the name of the resource the policy has to be applied to. This parameter can be omitted if **selector** is specified.
* In the **selector** parameter, you should specify conditions based on which the resources will be chosen for the policy to be applied to. This parameter is optional if **name** is specified. 
 
The **patch** sub-section defines what needs to be changed (i.e. mutated) before resource creation can take place. This section contains multiple entries of the path, operation, and value.

* In the **path** parameter, you should specify the required path. 
* In the **op** parameter, you should specify the required operation (Add | Replace | Delete).
* In the **value** parameter, you should specify either a number, a YAML string, or text. 

### 2. Adding secret and config map to namespace
```
apiVersion : policy.nirmata.io/v1alpha1
kind : Policy
metadata : 
  name : policy-namespace-default
spec :
  failurePolicy: stopOnError
  rules:
    - resource:
        kind : Namespace    
        name :
        selector :
          matchLabels :
           target: "production"
      configMapGenerator :
        name: game-config-env-file
        copyFrom: 
          namespace: some-ns
          name: some-other-config-map
        data:
          foo: bar
          app.properties: /
            foo1=bar1
            foo2=bar2
          ui.properties: /
            foo1=bar1
            foo2=bar2
      secretGenerator :
        name: game-secrets
        copyFrom: 
          namespace: some-ns
          name: some-other-secrets
        data: # data is optional
  ```
In this example, the **rules** section has the mandatory **resource** sub-section, additional **secretGenerator** and **configMapGenerator** sub-sections, and no optional **patch** sub-section.

The **configMapGenerator** sub-section defines the contents of the config-map which will be created in the future namespace.

The **copyFrom** parameter contains information about template config-map. The **data** parameter describes the contents of the created config-map. **copyFrom** and **data** are optional, but at least one of these fields must be specified. If both **copyFrom** and **data** are specified, then the template **copyFrom** will be used for the configuration, and then the specified **data** will be added to the config-map.

**secretGenerator** acts exactly as **configMapGenerator**, but creates a secret instead of the config-map.

### 3. More examples
An example of a policy that uses all available features: `definitions/policy-example.yaml`.

See the contents of `/examples`: there are definitions and policies for every supported type of resource.

# Build

## Prerequisites

You need to have the go installed and configured on your machine: [golang installation](https://golang.org/doc/install).
Ensure that the GOPATH environment variable is set to the desired location (usually `~/go`).

We are using [dep](https://github.com/golang/dep) **to resolve dependencies**.

We are using [goreturns](https://github.com/sqs/goreturns) **to format the sources** before commit.

Code generation for the CRD controller depends on kubernetes/hack, so before using code generation, execute:

`go get k8s.io/kubernetes/hack`

## Cloning

`git clone https://github.com/nirmata/kube-policy.git $GOPATH/src/github.com/nirmata/kube-policy`

Make sure that you use exactly the same subdirectory of the `$GOPATH` as shown above.

## Restore dependencies

Navigate to the kube-policy project dir and execute the following command:
`dep ensure`

This will install the necessary dependencies described in Gopkg.toml

## Compiling

We are using the code generator for the custom resource objects from here: https://github.com/kubernetes/code-generator

Generate additional controller code before compiling the project:

`scripts/update-codegen.sh`

Then you can build the controller:

`go build .`

# Installation

The controller can be installed and operated in two ways: **Outside the cluster** and **Inside the cluster**. The controller **outside** the cluster is much more convenient to debug and verify changes in its code, so we can call it 'debug mode'. The controller **inside** the cluster is designed for use in the real world, and the **QA testing** should be performed when controller operate in this mode.

## Outside the cluster (debug mode)

To run controller in this mode you should prepare TLS key/certificate pair for webhook, which will run on localhost and explicitly provide these files with kubeconfig to the controller.

1. Open your `~/.kube/config` file and copy the value of `certificate-authority-data` to the clipboard.
2. Open `crd/MutatingWebhookConfiguration_local.yaml` and replace `${CA_BUNDLE}` with the contents of the clipboard.
3. Open `~/.kube/config` again and copy the IP of the `server` value, for example `192.168.10.117`.
4. Run `scripts/deploy-controller.sh --service=localhost --serverIp=<server_IP>`, where `<server_IP>` is a server from the clipboard. This scripts will generate TLS certificate for webhook server and register this webhook in the cluster. Also it registers CustomResource `Policy`.
5. Start the controller using the following command: `sudo kube-policy --cert=certs/server.crt --key=certs/server-key.pem --kubeconfig=~/.kube/config`

## Inside the cluster (normal use)

Just execute the command for creating all necesarry stuff:

`kubectl create -f definitions/install.yaml`

In this mode controller will get TLS key/certificate pair and loads in-cluster config automatically on start.
If your worker node is equal to the master node, you will probably get such kind of error:

`... 1 node(s) had taints that the pod didn't tolerate ...`

In this case execute the command:

`kubectl taint nodes --all node-role.kubernetes.io/master-`

and run installation command again.