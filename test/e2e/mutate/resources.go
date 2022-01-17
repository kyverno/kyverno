package mutate

import (
	"fmt"

	"github.com/kyverno/kyverno/test/e2e"
)

var podGVR = e2e.GetGVR("", "v1", "pods")
var deploymentGVR = e2e.GetGVR("apps", "v1", "deployments")

func newNamespaceYaml(name string) []byte {
	ns := fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: %s
`, name)

	return []byte(ns)
}

// Cluster Policy to copy the copy me label from one configmap to the target
var configMapMutationYaml = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "mutate-policy"
spec:
  rules:
  - name: "gen-role"
    match:
      resources:
        kinds:
          - ConfigMap
    context:
    - name: labelValue
      apiCall:
        urlPath: "/api/v1/namespaces/{{ request.object.metadata.namespace }}/configmaps"
        jmesPath: "items[*]"
    mutate:
      patchStrategicMerge:
        metadata:
          labels:
            +(kyverno.key/copy-me): "{{ labelValue[?metadata.name == 'source'].metadata.labels.\"kyverno.key/copy-me\" | [0] }}"
`)

var configMapMutationWithContextLogicYaml = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "mutate-policy"
spec:
  rules:
  - name: "gen-role"
    match:
      resources:
        kinds:
          - ConfigMap
    context:
    - name: labelValue
      apiCall:
        urlPath: "/api/v1/namespaces/{{ request.object.metadata.namespace }}/configmaps"
        jmesPath: "items[?metadata.name == 'source'].metadata.labels.\"kyverno.key/copy-me\" | [0]"
    mutate:
      patchStrategicMerge:
        metadata:
          labels:
            +(kyverno.key/copy-me): "{{ labelValue }}"
`)

var configMapMutationWithContextLabelSelectionYaml = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "mutate-policy"
spec:
  rules:
  - name: "gen-role"
    match:
      resources:
        kinds:
          - ConfigMap
    context:
    - name: labelValue
      apiCall:
        urlPath: "/api/v1/namespaces/{{ request.object.metadata.namespace }}/configmaps"
        jmesPath: "items[?metadata.name == '{{ request.object.metadata.labels.\"kyverno.key/copy-from\" }}'].metadata.labels.\"kyverno.key/copy-me\" | [0]"
    mutate:
      patchStrategicMerge:
        metadata:
          labels:
            +(kyverno.key/copy-me): "{{ labelValue }}"
`)

// Source ConfigMap from which data is taken to copy
var sourceConfigMapYaml = []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: source
  namespace: test-mutate
  labels:
    kyverno.key/copy-me: sample-value
data:
  data.yaml: |
    some: data
`)

// Target ConfigMap which is mutated
var targetConfigMapYaml = []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: target
  namespace: test-mutate
  labels:
    kyverno.key/copy-from: source
data:
  data.yaml: |
    some: data
`)

var mutateIngressCpol = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: mutate-ingress-host
spec:
  rules:
  - name: mutate-rules-host
    match:
      resources:
        kinds:
        - Ingress
        namespaces:
        - test-ingress
    mutate:
      patchesJson6902: |-
        - op: replace
          path: /spec/rules/0/host
          value: "{{request.object.spec.rules[0].host}}.mycompany.com"
`)

var ingressNetworkingV1 = []byte(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kuard-v1
  namespace: test-ingress
  labels:
    app: kuard
spec:
  rules:
  - host: kuard
    http:
      paths:
      - backend:
          service: 
            name: kuard
            port: 
              number: 8080
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - kuard
`)

var ingressNetworkingV1beta1 = []byte(`
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  labels:
    app: kuard
  name: kuard-v1beta1
  namespace: test-ingress
spec:
  rules:
  - host: kuard
    http:
      paths:
      - backend:
          serviceName: kuard
          servicePort: 8080
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - kuard
`)

var setRunAsNonRootTrue = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: set-runasnonroot-true
spec:
  rules:
    - name: set-runasnonroot-true
      match:
        resources:
          kinds:
          - Pod
      mutate:
        patchStrategicMerge:
          spec:
            securityContext:
              runAsNonRoot: true
            initContainers:
              - (name): "*"
                securityContext:
                  runAsNonRoot: true
            containers:
              - (name): "*"
                securityContext:
                  runAsNonRoot: true
`)

var podWithContainers = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: foo
  namespace: test-mutate
  labels:
    app: foo
spec:
  containers:
  - image: abc:1.28
    name: busybox
`)

var podWithContainersPattern = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: foo
  namespace: test-mutate
  labels:
    app: foo
spec:
  securityContext:
    runAsNonRoot: true
  containers:
  - (name): "*"
    securityContext:
      runAsNonRoot: true
`)

var podWithContainersAndInitContainers = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: foo
  namespace: test-mutate1
  labels:
    app: foo
spec:
  containers:
  - image: abc:1.28
    name: busybox
  initContainers:
  - image: bcd:1.29
    name: nginx
`)

var podWithContainersAndInitContainersPattern = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: foo
  namespace: test-mutate1
  labels:
    app: foo
spec:
  securityContext:
    runAsNonRoot: true
  containers:
  - (name): "*"
    securityContext:
      runAsNonRoot: true
  initContainers:
  - (name): "*"
    securityContext:
      runAsNonRoot: true
`)

var kyverno_mutate_json_patch = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: add-image-as-env-var
  # env array needs to exist (least one env var is present)
  annotations:
    pod-policies.kyverno.io/autogen-controllers: None
    policies.kyverno.io/title: Add Image as Environment Variable
    policies.kyverno.io/category: Other
    policies.kyverno.io/severity: medium
    policies.kyverno.io/minversion: 1.4.3
    policies.kyverno.io/subject: Pod, Deployment
    policies.kyverno.io/description: >-
      The Kubernetes downward API only has the ability to express so many
      options as environment variables. The image consumed in a Pod is commonly
      needed to make the application aware of some logic it must take. This policy
      takes the value of the `image` field and adds it as an environment variable
      to bare Pods and Deployments having no more than two containers. The `env` array must already exist for the policy
      to operate correctly. This policy may be easily extended to support other higher-level
      Pod controllers as well as more containers by following the established rules.      
spec:
  background: false
  schemaValidation: false
  rules:
  # One Pod
  - name: pod-containers-1-inject-image
    match:
      resources:
        kinds:
        - Pod
    preconditions:
      all:
      - key: "{{request.object.spec.containers[] | length(@)}}"
        operator: GreaterThanOrEquals
        value: 1
    mutate:
      patchesJson6902: |-
        - op: add
          path: "/spec/containers/0/env/-"
          value: {"name":"K8S_IMAGE","value":"{{request.object.spec.containers[0].image}}"}        
  # Two or more Pods
  - name: pod-containers-2-inject-image
    match:
      resources:
        kinds:
        - Pod
    preconditions:
      all:
      - key: "{{request.object.spec.containers[] | length(@)}}"
        operator: GreaterThanOrEquals
        value: 2
    mutate:
      patchesJson6902: |-
        - op: add
          path: "/spec/containers/1/env/-"
          value: {"name":"K8S_IMAGE","value":"{{request.object.spec.containers[1].image}}"}        
  # Deployment with one Pod
  - name: deploy-containers-1-inject-image
    match:
      resources:
        kinds:
        - Deployment
    preconditions:
      all:
      - key: "{{request.object.spec.template.spec.containers[] | length(@)}}"
        operator: GreaterThanOrEquals
        value: 1
    mutate:
      patchesJson6902: |-
        - op: add
          path: "/spec/template/spec/containers/0/env/-"
          value: {"name":"K8S_IMAGE","value":"{{request.object.spec.template.spec.containers[0].image}}"}        
  # Deployment with two or more Pods
  - name: deploy-containers-2-inject-image
    match:
      resources:
        kinds:
        - Deployment
    preconditions:
      all:
      - key: "{{request.object.spec.template.spec.containers[] | length(@)}}"
        operator: GreaterThanOrEquals
        value: 2
    mutate:
      patchesJson6902: |-
        - op: add
          path: "/spec/template/spec/containers/1/env/-"
          value: {"name":"K8S_IMAGE","value":"{{request.object.spec.template.spec.containers[1].image}}"}
`)

var podWithEnvVar = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: foo
spec:
  initContainers:
  - name: jimmy
    image: defdasdabian:923
    command: ["/bin/sh", "-c", "sleep infinity"]
  containers:
  - name: foo
    image: defdasdabian:923
    command: ["/bin/sh", "-c", "sleep infinity"]
    env:
      - name: FOO
        value: bar
  - name: asdf
    image: asdfasdfasdf:123
    command: ["/bin/sh", "-c", "sleep infinity"]
	  env:
	    - name: FOO
	      value: bar
`)

var podWithEnvVarPattern = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: foo
	spec:
	initContainers:
	- (name): "*"
	  command: ["/bin/sh", "-c", "sleep infinity"]
	  securityContext:
		  capabilities:
		    drop:
		    - NET_RAW
		    - SETUID
	containers:
	- (name): "*"
	  command: ["/bin/sh", "-c", "sleep infinity"]
	  securityContext:
		  capabilities:
		    drop:
		    - SETUID
		    - CAP_FOO_BAR
		    - NET_RAW
	  env:
		  - name: FOO
		    value: bar
	- (name): "*"
	  command: ["/bin/sh", "-c", "sleep infinity"]
	  securityContext:
	    capabilities:
	      drop:
	      - NET_RAW
	      - SOME_THING
    env:
	    - name: FOO
	      value: bar
`)

var kyverno_2316_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: structured-logs-sidecar
spec:
  background: false
  rules:
    - name: add-annotations
      match:
        resources:
          kinds:
            - Deployment
          annotations:
            structured-logs: "true"
      mutate:
        patchStrategicMerge:
          metadata:
            annotations:
              "fluentbit.io/exclude-{{request.object.spec.template.spec.containers[0].name}}": "true"
`)

var kyverno_2316_resource = []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busybox
  namespace: test-mutate2
  annotations:
    structured-logs: "true"
  labels:
    # app: busybox
    color: red
    animal: bear
    food: pizza
    car: jeep
    env: qa
    # foo: blaaah
spec:
  replicas: 1
  selector:
    matchLabels:
      app: busybox
  template:
    metadata:
      labels:
        app: busybox
        # foo: blaaah
    spec:
      containers:
      - image: busybox:1.28
        name: busybox
        command: ["sleep", "9999"]
        resources:
          requests:
            cpu: 100m
            memory: 10Mi
          limits:
            cpu: 100m
            memory: 10Mi
      - image: busybox:1.28
        name: busybox1
        command: ["sleep", "9999"]
        resources:
          requests:
            cpu: 100m
            memory: 10Mi
          limits:
            cpu: 100m
            memory: 20Mi
`)

var kyverno_2316_pattern = []byte(`
metadata:
  annotations:
    fluentbit.io/exclude-busybox: "true"
`)

var kyverno_2971_policy = []byte(`
apiVersion : kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: replace-docker-hub
spec:
  rules:
  - name: replace-docker-hub
    match:
      resources:
        kinds:
        - Pod
    preconditions:
      all:
      - key: "{{request.operation}}"
        operator: In
        value:
        - CREATE
        - UPDATE
    mutate:
      foreach:
      - list: "request.object.spec.containers"
        preconditions:
          all:
            - key: '{{images.containers."{{element.name}}".registry}}'
              operator: Equals
              value: 'docker.io'
        patchStrategicMerge:
          spec:
            containers:
            - name: "{{ element.name }}"           
              image: 'my-private-registry/{{images.containers."{{element.name}}".path}}:{{images.containers."{{element.name}}".tag}}'
`)

var kyverno_2971_resource = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: test-mutate
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
`)

var kyverno_2971_pattern = []byte(`
spec:
  containers:
  - name: "nginx"           
    image: 'my-private-registry/nginx:1.14.2'
`)
