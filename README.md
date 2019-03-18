# kube-policy
A Kubernetes native policy engine

## Motivation

## How it works
The solution provides a possibility to validate the custom Kubernetes resources and modify them before their creation. 
### Components
* **Policy Controller** (`/controller`) allows defining custom resources which can be used in your Kubernetes cluster
* **WebHooks Server** (`/server`) implements the connection between Kubernetes API server and **Mutation WebHook**
* **Mutation WebHook** (`/webhooks`) allows applying Nirmata policies for validation and mutation of the certain types of resources (see the list below)
* **Kube Client** (`/kubeclient`) allows other components to communicate with Kubernetes API server for resource management in a cluster
* **Initialization functions** (`/init.go`, `/utils`) allow running the controller inside the cluster without deep pre-tuning

The program initializes the configuration of the client API Cubernetis and creates a HTTPS server with a webhook for resource mutation. When a resource is created in a cluster for various reasons, the Kerbernetes core sends a request for a mutation of this resource to the web hook. The policy controller manages the objects of the politicians created in the cluster and is always aware of what policies are currently in effect: information about the policies is available on the webhook thanks to the policy controller. The request to create a resource contains its full definition. If the resource matches one or more of the current policies, the resource mutates in accordance with them.

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

**NOTE**: **add** and **replace** behave in the same way, so they can be used interchangeably. But there is the difference between 'add' and 'replace' operations in case of mutating an array. In this case 'add' operation will add an element to the list 'replace' operation replaces whole list.

After the resource YAMP file is validated and mutated, the required object is created in the Kubernetes cluster. 

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
The **failurePolicy** parameter is optional. It is set to **stopOnError** by default. Other possible value is **continueOnError**. If **continueOnError** is specified, the resource will be created despite the errors occured in web hook.
The **rules** section consists of mandatory **resource** sub-section and optional **patch** sub-section.
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
The **rules** section in this example have mandatory **resource** sub-section, additional **secretGenerator** and **configMapGenerator** sub-sections, and has no and optional **patch** sub-section.
**configMapGenerator** sub-section defines the contents of the config-map which will be created in future namespace.
**copyFrom** contains information about template config-map. **data** describes the contents of created config-map. **copyFrom** and **data** are optional, but at least one of these fields must be specified. If both the **copyFrom** and the **data** are specified, then the template **copyFrom** will be used for the configuration, and then the specified **data** will be added to config-map.
**secretGenerator** acts exactly as **configMapGenerator**, but creates the secret insted of config-map.

### More examples
See the contents of `/examples`: there are definitions and policies for every supported type of resource.

# Build

## Prerequisites

You need to have the go and dep utils installed on your machine.
Ensure that GOPATH environment variable is set to the desired location.
Code generation for the CRD controller depends on kubernetes/hack, so before using code generation, execute:

`go get k8s.io/kubernetes/hack`

We are using [dep](https://github.com/golang/dep)

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

There are two possible ways of installing and using the controller: for **development** and for **production**

## For development

_At the time of creation of these instructions, only this installation method worked_

1. Open your `~/.kube/config` file and copy the value of `certificate-authority-data` to the clipboard.
2. Open `crd/MutatingWebhookConfiguration_local.yaml` and replace `${CA_BUNDLE}` with the contents of the clipboard.
3. Open `~/.kube/config` again and copy the IP of the `server` value, for example `192.168.10.117`.
4. Run `scripts/deploy-controller.sh --service=localhost --serverIp=<server_IP>`, where `<server_IP>` is a server from the clipboard. This scripts will generate TLS certificate for webhook server and register this webhook in the cluster. Also it registers CustomResource `Policy`.
5. Start the controller using the following command: `sudo kube-policy --cert=certs/server.crt --key=certs/server-key.pem --kubeconfig=~/.kube/config`

## For production

_To be implemented_
The scripts for "development installation method" will be moved to the controller's code. The solution will perform the preparation inside the cluster automatically. Yuo should be able to use `definitions/install.yaml` to install the controller.