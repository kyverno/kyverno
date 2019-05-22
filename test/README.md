# Test examples
Examples of policies and resources with which you can play to see the kube-policy in action. There are definitions for each supported resource type and an example policy for the corresponding resource.
## How to play
For now, the testing is possible only via ```kubectl``` when kyverno is installed to the cluster. So, [build and install the policy controller](/documentation/installation.md) first.

Each folder contains a pair of files, one of which is the definition of the resource, and the second is the definition of the policy for this resource. Let's look at an example of the endpoints mutation. Endpoints are listed in file `examples/Endpoints/endpoints.yaml`:

````yaml
apiVersion: v1
kind: Endpoints
metadata:
  name: test-endpoint
  labels:
    label : test
subsets:
- addresses:
  - ip: 192.168.10.171
  ports:
  - name: secure-connection
    port: 443
    protocol: TCP
````
Create this resource:

````yaml
> kubectl create -f test/Endpoints/endpoints.yaml
endpoints/test-endpoint created
> kubectl get -f test/Endpoints/endpoints.yaml
NAME            ENDPOINTS            AGE
test-endpoint   192.168.10.171:443   6s
````
We just created an endpoints resource and made sure that it was created without changes. Let's remove it now and try to create it again, but with an active policy for endpoints resources.
````bash
> kubectl delete -f test/Endpoints/endpoints.yaml
endpoints "test-endpoint" deleted
````
We have this a policy for enpoints ([policy-endpoint.yaml](/test/Endpoints/policy-endpoint.yaml)):

````yaml
apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy-endpoints
spec :
  rules:
    - name:
      resource:
        kinds:
          - Endpoints
        selector:
          matchLabels:
            label : test
      mutate:
        patches:
          - path : "/subsets/0/ports/0/port"
            op : replace
            value: 9663
          - path : "/subsets/0"
            op: add
            value:
              addresses:
              - ip: "192.168.10.171"
              ports:
              - name: load-balancer-connection
                port: 80
                protocol: UDP
````
This policy does 2 patches:

- **replaces** the first port of the first connection to 6443
- **adds** new endpoint with IP 192.168.10.171 and port 80 (UDP)

Let's apply this policy and create the endpoints again to see the changes:
````bash
> kubectl create -f test/Endpoints/policy-endpoints.yaml
policy.policy.nirmata.io/policy-endpoints created
> kubectl create -f test/Endpoints/endpoints.yaml
endpoints/test-endpoint created
> kubectl get -f test/Endpoints/endpoints.yaml
NAME            ENDPOINTS                               AGE
test-endpoint   192.168.10.171:80,192.168.10.171:9663   30s
````
As you can see, the endpoints resource was created with changes: a new port 80 was added, and port 443 was changed to 6443.

**Enjoy :)**
