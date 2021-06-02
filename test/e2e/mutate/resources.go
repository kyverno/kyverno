package mutate

import "fmt"

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
          value: {{request.object.spec.rules[0].host}}.mycompany.com
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

var ingressExtensionV1beta1 = []byte(`
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  labels:
    app: kuard
  name: kuard-extensions
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
