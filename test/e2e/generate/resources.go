package generate

var genClusterRoleYaml = []byte(`
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
        name: "ns-cluster-role"
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
        name: "ns-cluster-role-binding"
        data:
          roleRef:
            apiGroup: rbac.authorization.k8s.io
            kind: ClusterRole
            name: "ns-cluster-role"
          subjects:
          - kind: ServiceAccount
            name: "kyverno-service-account"
            namespace: test
`)

var namespaceYaml = []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: test
`)

var genRoleYaml = []byte(`
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
