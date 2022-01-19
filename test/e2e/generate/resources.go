package generate

// Namespace Description
var namespaceYaml = []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: test
`)

// Namespace With Label Description
var namespaceWithLabelYaml = []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: test
  labels:
    security: standard
`)

// Cluster Policy to generate Role and RoleBinding with synchronize=true
var roleRoleBindingYamlWithSync = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "gen-role-policy"
spec:
  background: false
  rules:
  - name: "gen-role"
    match:
        resources:
          kinds:
            - Namespace
    generate:
        kind: Role
        name: "ns-role"
        namespace: "{{request.object.metadata.name}}"
        synchronize: true
        data:
          rules:
          - apiGroups: [""]
            resources: ["pods"]
            verbs: ["get", "watch", "list"]
  - name: "gen-role-binding"
    match:
        resources:
          kinds:
            - Namespace
    generate:
        kind: RoleBinding
        name: "ns-role-binding"
        namespace: "{{request.object.metadata.name}}"
        synchronize: true
        data:
          subjects:
            - apiGroup: rbac.authorization.k8s.io
              kind: User
              name: minikube-user
          roleRef:
            kind: Role
            name: ns-role
            namespace: "{{request.object.metadata.name}}"
            apiGroup: rbac.authorization.k8s.io
`)

// Cluster Policy to generate Role and RoleBinding with Clone
var roleRoleBindingYamlWithClone = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "gen-role-policy"
spec:
  background: false
  rules:
  - name: "gen-role"
    match:
        resources:
          kinds:
           - Namespace
    generate:
        kind: Role
        name: "ns-role"
        namespace: "{{request.object.metadata.name}}"
        synchronize: true
        clone:
          name: "ns-role"
          namespace: "default"
  - name: "gen-role-binding"
    match:
        resources:
          kinds:
           - Namespace
    generate:
        kind: RoleBinding
        name: "ns-role-binding"
        namespace: "{{request.object.metadata.name}}"
        synchronize: true
        clone:
            name: "ns-role-binding"
            namespace: default
`)

// Source Role from which ROle is Cloned by generate
var sourceRoleYaml = []byte(`
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: default
  name: ns-role
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "watch", "list", "delete", "create"]
`)

// Source RoleBinding from which RoleBinding is Cloned by generate
var sourceRoleBindingYaml = []byte(`
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: ns-role-binding
  namespace: default
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: minikube-user
roleRef:
  kind: Role
  name: ns-role
  apiGroup: rbac.authorization.k8s.io
`)

// ClusterPolicy to generate ClusterRole and ClusterRoleBinding with synchronize = true
var genClusterRoleYamlWithSync = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "gen-cluster-policy"
spec:
  background: false
  rules:
  - name: "gen-cluster-role"
    match:
       resources:
         kinds:
         - Namespace
    generate:
        kind: ClusterRole
        name: ns-cluster-role
        synchronize: true
        data:
          rules:
          - apiGroups: [""]
            resources: ["pods"]
            verbs: ["get", "watch", "list"]
  - name: "gen-cluster-role-binding"
    match:
       resources:
         kinds:
         - Namespace
    generate:
        kind: ClusterRoleBinding
        name: ns-cluster-role-binding
        synchronize: true
        data:
          roleRef:
            apiGroup: rbac.authorization.k8s.io
            kind: ClusterRole
            name: ns-cluster-role
          subjects:
          - kind: ServiceAccount
            name: "kyverno-service-account"
            namespace: "{{request.object.metadata.name}}"
`)

var baseClusterRoleData = []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: base-cluster-role
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
  - roles
  - clusterroles
  - rolebindings
  - clusterrolebindings
  verbs:
  - create # generate new resources
  - get # check the contents of exiting resources
  - update # update existing resource, if required configuration defined in policy is not present
  - delete # clean-up, if the generate trigger resource is deleted
`)

var baseClusterRoleBindingData = []byte(`
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: base-cluster-role-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: base-cluster-role
subjects:
- kind: ServiceAccount
  name: kyverno-service-account
  namespace: kyverno
`)

var genNetworkPolicyYaml = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: add-networkpolicy
spec:
  background: true
  rules:
    - name: allow-dns
      match:
        resources:
          kinds:
            - Namespace
          selector:
            matchLabels:
              security: standard
      exclude:
        resources:
          namespaces:
            - "kube-system"
            - "default"
            - "kube-public"
            - "nova-kyverno"
      generate:
        synchronize: true
        kind: NetworkPolicy
        name: allow-dns
        namespace: "{{request.object.metadata.name}}"
        data:
          spec:
            egress:
              - ports:
                  - protocol: UDP
                    port: 5353
            podSelector: {}
            policyTypes:
              - Egress
`)

var updatGenNetworkPolicyYaml = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: add-networkpolicy
spec:
  background: true
  rules:
    - name: allow-dns
      match:
        resources:
          kinds:
            - Namespace
          selector:
            matchLabels:
              security: standard
      exclude:
        resources:
          namespaces:
            - "kube-system"
            - "default"
            - "kube-public"
            - "nova-kyverno"
      generate:
        synchronize: true
        kind: NetworkPolicy
        name: allow-dns
        namespace: "{{request.object.metadata.name}}"
        data:
          spec:
            egress:
              - ports:
                  - protocol: TCP
                    port: 5353
            podSelector: {}
            policyTypes:
              - Egress
`)

var updateSynchronizeInGeneratePolicyYaml = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: add-networkpolicy
spec:
  background: true
  rules:
    - name: allow-dns
      match:
        resources:
          kinds:
            - Namespace
          selector:
            matchLabels:
              security: standard
      exclude:
        resources:
          namespaces:
            - "kube-system"
            - "default"
            - "kube-public"
            - "nova-kyverno"
      generate:
        synchronize: false
        kind: NetworkPolicy
        name: allow-dns
        namespace: "{{request.object.metadata.name}}"
        data:
          spec:
            egress:
              - ports:
                  - protocol: UDP
                    port: 5353
            podSelector: {}
            policyTypes:
              - Egress
`)

var cloneSourceResource = []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: game-demo
data:
  initial_lives: "2"
`)

var genCloneConfigMapPolicyYaml = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: generate-policy
spec:
  rules:
  - name: copy-game-demo
    match:
      resources:
        kinds:
        - Namespace
    exclude:
      resources:
        namespaces:
        - kube-system
        - default
        - kube-public
        - kyverno
    generate:
      kind: ConfigMap
      name: game-demo
      namespace: "{{request.object.metadata.name}}"
      synchronize: true
      clone:
        namespace: default
        name: game-demo
`)
