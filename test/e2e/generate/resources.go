package generate

// Namespace Description
var namespaceYaml = []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: test
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
        namespace: test
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
        namespace: test
        synchronize: true
        data:
          subjects:
            - apiGroup: rbac.authorization.k8s.io
              kind: User
              name: minikube-user
          roleRef:
            kind: Role
            name: ns-role
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
        namespace: test
        synchronize: true
        clone:
              kind: Role
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
        namespace: test
        synchronize: true
        clone:
            kind: RoleBinding
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
- apiGroups: ["*"]
  resources: ["*"]
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
            namespace: test
`)
