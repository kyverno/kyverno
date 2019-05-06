# kube-policy
A Kubernetes native policy engine

### Workflow

We work using [bitbucket gitflow](https://www.atlassian.com/git/tutorials/comparing-workflows): the work is performed in separate branches that merge into the develop branch, and the current stable version of the product is in the master branch. Working with the repository looks like this:

1. Create an issue, add specification and architecture, discuss with teammates

2. From the develop branch create your own branch named as the issue with its number. For example, the branch for the [issue #14](https://github.com/nirmata/kube-policy/issues/14) should have a name *14-Events_support*.

3. Work only with your own branch, create pull requests to the develop branch after each stage of development (about the stages see below).

4. Create a final pull request to the develop branch when the work on the issue is done. The pull request must meet the **requirements**:
* Should be able to merge without conflicts. To resolve conflicts in pull request, merge the actual develop branch to your branch and update the request.
* Should not contain non-compiled code. After each pull request, the develop branch must be buildable, because at any time, anyone from the team can create feature branch from the develop.
* Should not have unused and commented code, as well as unused resources.
* Should be confirmed by an actual Development Manager of the project.

5. Before creating a new version of the product, the develop branch is merged to the master. The master should not have direct commits from branches other than develop.

Before starting a task, we need to think it over well and create the only right solution that will allow us to achieve the goal without compromising the existing functionality. The process of preparing for the task is divided into stages:
1. **Strict specifications**.
Before writing the code, an exact feature specification must be written to the issue and discussed. We must work exactly according to the specification and not change it on the go to eliminate confusion between team members.

2. **Architectures**.
For complex features, we create architectures in the form of UML diagrams. Diagrams visualize your thoughts, as well as specifications, but provide more details to the developer. With the help of diagrams, we solve possible problems before they arise. Also, using diagrams, you can divide the feature into components and schedule individual tasks for their implementation.

3. **Separation**.
Each issue describes a global task that can be divided into subtasks. When the big issue already has a clear specification and, possibly, an architecture, it can be divided into separate stages and be carried out sequentially. The stage of performing the task is the merge to develop branch. You can create a branch for each stage, but you can also create pull requests sequentially from the same branch that is created for your issue. **We do not mix work on several tasks in one branch.**

Following these simple rules, we can work effectively without interfering with each other, and keep the project in the best possible shape all the time.

## How it works
The solution provides a possibility to validate Kubernetes resources and modify them before their creation. 
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

Just execute the command for creating all necesarry resources:
`kubectl create -f definitions/install.yaml`

In this mode controller will get TLS key/certificate pair and loads in-cluster config automatically on start.
To check if the controller is working, find it in the list of kube-system pods:

`kubectl get pods -n kube-system`

The pod with controller contains **'kube-policy'** in its name. The STATUS column will show the health state of the controller. If controller doesn't start, see its logs:

`kubectl describe pod <kube-policy-pod-name> -n kube-system`

or

`kubectl logs <kube-policy-pod-name> -n kube-system`

### Troubleshuting

**1. Pulling image from private repo**

_This issue is always actual for now._

If the kube-policy image is in private repo, you should probably see **ImagePullBackOff** as a STATUS of controller's pod. That's because cluster lacks credentials for this repo. To add credentials to the cluster, do the next steps:
1. Delete previous installation:

    `kubectl delete -f definitions/install.yaml`

    `kubectl delete MutatingWebhookConfiguration nirmata-kube-policy-webhook-cfg`

2. Login to docker:

    `docker login`
    
    This will create `~/.docker/config.json` file with credentials
    
3. Save docker credentials to the secret:

    `DOCKER_CREDS="$(base64 ~/.docker/config.json) -w 0"`
    
    `sed "s,DOCKER_CONFIG_JSON_IN_BASE64,$DOCKER_CREDS,g" definitions/docker-registry-key.yaml > definitions/docker-creds.yaml`
    
    `kubectl create -f definitions/docker-creds.yaml`
    
4. Install controller again

    `kubectl create -f definitions/install.yaml`

**2. Taints problem when running on master node**

If your worker node is equal to the master node, and controller doesn't start, you will probably see such kind of error in its logs:

`... 1 node(s) had taints that the pod didn't tolerate ...`

To fix it execute the command:

`kubectl taint nodes --all node-role.kubernetes.io/master-`

And reinstall the controller:

`kubectl delete -f definitions/install.yaml`
    
`kubectl create -f definitions/install.yaml`
