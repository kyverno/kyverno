package mutate

import (
	"fmt"

	"github.com/kyverno/kyverno/test/e2e"
)

var podGVR = e2e.GetGVR("", "v1", "pods")
var deploymentGVR = e2e.GetGVR("apps", "v1", "deployments")
var configmGVR = e2e.GetGVR("", "v1", "configmaps")
var secretGVR = e2e.GetGVR("", "v1", "secrets")

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
      takes the value of the 'image' field and adds it as an environment variable
      to bare Pods and Deployments having no more than two containers. The 'env' array must already exist for the policy
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
  namespace: test-mutate-env-array
spec:
  containers:
  - command:
    - sleep infinity
    env:
    - name: K8S_IMAGE
      value: docker.io/busybox:1.11
    image: busybox:1.11
    name: busybox
    securityContext:
      capabilities:
        drop:
        - SETUID
  initContainers:
  - command:
    - sleep infinity
    image: nginx:1.14
    name: nginx
    securityContext:
      capabilities:
        drop:
        - SETUID
`)

var podWithEnvVarPattern = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: foo
  namespace: test-mutate-env-array
spec:
  containers:
  - command:
    - sleep infinity
    env:
    - name: K8S_IMAGE
      value: docker.io/busybox:1.11
    image: busybox:1.11
    name: busybox
    securityContext:
      capabilities:
        drop:
        - SETUID
  - command:
    - sleep infinity
    env:
    - name: K8S_IMAGE
      value: linkerd:1.21
    image: linkerd:1.21
    name: linkerd
    securityContext:
      capabilities:
        drop:
        - NET_RAW
        - SOME_THING
  initContainers:
  - command:
    - sleep infinity
    image: nginx:1.14
    name: nginx
    securityContext:
      capabilities:
        drop:
        - SETUID
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
  namespace: test-mutate-img
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

var annotate_host_path_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata: 
  name: add-safe-to-evict
  annotations:
    policies.kyverno.io/category: Workload Management
    policies.kyverno.io/description: The Kubernetes cluster autoscaler does not evict pods that 
      use hostPath or emptyDir volumes. To allow eviction of these pods, the annotation 
      cluster-autoscaler.kubernetes.io/safe-to-evict=true must be added to the pods. 
spec: 
  rules: 
  - name: annotate-empty-dir
    match:
      resources:
        kinds:
        - Pod
    mutate:
      patchStrategicMerge:
        metadata:
          annotations:
            +(cluster-autoscaler.kubernetes.io/safe-to-evict): "true"
        spec:          
          volumes: 
          - <(emptyDir): {}
  - name: annotate-host-path
    match:
      resources:
        kinds:
        - Pod
    mutate:
      patchStrategicMerge:
        metadata:
          annotations:
            +(cluster-autoscaler.kubernetes.io/safe-to-evict): "true"
        spec:          
          volumes: 
          - hostPath:
              <(path): "*"
`)

var podWithEmptyDirAsVolume = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: pod-with-emptydir
  namespace: emptydir
  labels:
    foo: bar
spec:
  containers:
  - image: nginx
    name: nginx
    volumeMounts:
    - mountPath: /cache
      name: cache-volume
  volumes:
  - name: cache-volume
    emptyDir: {}
`)

var podWithVolumePattern = []byte(`
metadata:
  annotations:
    cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
`)

var podWithHostPathAsVolume = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: pod-with-hostpath
  namespace: hostpath
  labels:
    foo: bar
spec:
  containers:
  - image: nginx
    name: nginx
    volumeMounts:
    - mountPath: /usr/share/nginx/html
      name: test-volume
  volumes:
  - hostPath:
      path: /var/local/aaa
      type: DirectoryOrCreate
    name: test-volume
`)

var policyCreateTrigger = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "test-post-mutation-create-trigger"
spec:
  mutateExistingOnPolicyUpdate: false
  rules:
    - name: "mutate-deploy-on-configmap-create"
      match:
        any:
        - resources:
            kinds:
            - ConfigMap
            names:
            - dictionary-1
            namespaces:
            - staging-1
      mutate:
        targets:
        - apiVersion: v1
          kind: Secret
          name: test-secret-1
          namespace: "{{ request.object.metadata.namespace }}"
        patchStrategicMerge:
          metadata:
            labels:
              foo: "{{ request.object.metadata.name }}"
`)

var triggerCreateTrigger = []byte(`
apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  labels:
    test: createTrigger
  name: dictionary-1
  namespace: staging-1
`)

var targetCreateTrigger = []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-1
  namespace: staging-1
  labels:
    test: createTrigger
type: Opaque
data:
  value: Z29vZGJ5ZQ==
`)

var expectedTargetCreateTrigger = []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-1
  namespace: staging-1
  labels:
    test: createTrigger
    foo: dictionary-1
type: Opaque
data:
  value: Z29vZGJ5ZQ==
`)

var policyDeleteTrigger = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "test-post-mutation-delete-trigger"
spec:
  mutateExistingOnPolicyUpdate: false
  rules:
    - name: "mutate-deploy-on-configmap-delete"
      match:
        any:
        - resources:
            kinds:
            - ConfigMap
            names:
            - dictionary-2
            namespaces:
            - staging-2
      preconditions:
        any:
        - key: "{{ request.operation }}"
          operator: Equals
          value: DELETE
      mutate:
        targets:
        - apiVersion: v1
          kind: Secret
          name: test-secret-2
          namespace: "{{ request.object.metadata.namespace }}"
        patchStrategicMerge:
          metadata:
            labels:
              foo: "{{ request.object.metadata.name }}"
`)

var triggerDeleteTrigger = []byte(`
apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  labels:
    test: deleteTrigger
  name: dictionary-2
  namespace: staging-2
`)

var targetDeleteTrigger = []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-2
  namespace: staging-2
  labels:
    test: deleteTrigger
type: Opaque
data:
  value: Z29vZGJ5ZQ==
`)

var expectedTargetDeleteTrigger = []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-2
  namespace: staging-2
  labels:
    test: deleteTrigger
    foo: dictionary-2
type: Opaque
data:
  value: Z29vZGJ5ZQ==
`)

var policyCreatePolicy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "test-post-mutation-create-policy"
spec:
  mutateExistingOnPolicyUpdate: true
  rules:
    - name: "mutate-deploy-on-policy-create"
      match:
        any:
        - resources:
            kinds:
            - ConfigMap
            names:
            - dictionary-3
            namespaces:
            - staging-3
      mutate:
        targets:
        - apiVersion: v1
          kind: Secret
          name: test-secret-3
          namespace: "{{ request.object.metadata.namespace }}"
        patchStrategicMerge:
          metadata:
            labels:
              foo: "{{ request.object.metadata.name }}"
`)

var triggerCreatePolicy = []byte(`
apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  labels:
    test: createPolicy
  name: dictionary-3
  namespace: staging-3
`)

var targetCreatePolicy = []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-3
  namespace: staging-3
  labels:
    test: createPolicy
type: Opaque
data:
  value: Z29vZGJ5ZQ==
`)

var expectedTargetCreatePolicy = []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-3
  namespace: staging-3
  labels:
    test: createPolicy
    foo: dictionary-3
type: Opaque
data:
  value: Z29vZGJ5ZQ==
`)

var policyCreateTriggerJsonPatch = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "test-post-mutation"
spec:
  mutateExistingOnPolicyUpdate: false
  rules:
    - name: "mutate-deploy-on-configmap-update"
      match:
        any:
        - resources:
            kinds:
            - ConfigMap
            names:
            - dictionary-4
            namespaces:
            - staging-4
      mutate:
        targets:
        - apiVersion: v1
          kind: Secret
          name: test-secret-4
          namespace: "{{ request.object.metadata.namespace }}"
        patchesJson6902: |-
          - op: add
            path: "/metadata/labels/env"
            value: "{{ request.object.metadata.namespace }}"  
`)

var triggerCreateTriggerJsonPatch = []byte(`
apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  labels:
    test: createTrigger
  name: dictionary-4
  namespace: staging-4
`)

var targetCreateTriggerJsonPatch = []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-4
  namespace: staging-4
  labels:
    test: createTrigger
type: Opaque
data:
  value: Z29vZGJ5ZQ==
`)

var expectedCreateTriggerJsonPatch = []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-4
  namespace: staging-4
  labels:
    test: createTrigger
    env: staging-4
type: Opaque
data:
  value: Z29vZGJ5ZQ==
`)
