package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPolicy(t *testing.T) {
	type args struct {
		bytes []byte
	}
	type policy struct {
		kind      string
		namespace string
	}
	tests := []struct {
		name                            string
		args                            args
		wantPolicies                    []policy
		vaps                            []policy
		vapBindings                     []policy
		MutatingAdmissionPolicies       []policy
		MutatingAdmissionPolicyBindings []policy
		wantErr                         bool
	}{{
		name: "policy",
		args: args{
			[]byte(`
apiVersion: kyverno.io/v1
kind: Policy
metadata:
  name: generate-policy
  namespace: ns-1
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
`),
		},
		wantPolicies: []policy{
			{"Policy", "ns-1"},
		},
		wantErr: false,
	}, {
		name: "policy without ns",
		args: args{
			[]byte(`
apiVersion: kyverno.io/v1
kind: Policy
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
`),
		},
		wantPolicies: []policy{
			{"Policy", "default"},
		},
		wantErr: false,
	}, {
		name: "cluster policy",
		args: args{
			[]byte(`
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
`),
		},
		wantPolicies: []policy{
			{"ClusterPolicy", ""},
		},
		wantErr: false,
	}, {
		name: "cluster policy with ns",
		args: args{
			[]byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: generate-policy
  namespace: ns-1
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
`),
		},
		wantPolicies: []policy{
			{"ClusterPolicy", ""},
		},
		wantErr: false,
	}, {
		name: "policy and cluster policy",
		args: args{
			[]byte(`
apiVersion: kyverno.io/v1
kind: Policy
metadata:
  name: generate-policy
  namespace: ns-1
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
---
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
`),
		},
		wantPolicies: []policy{
			{"Policy", "ns-1"},
			{"ClusterPolicy", ""},
		},
		wantErr: false,
	}, {
		name: "policy and cluster policy in list",
		args: args{
			[]byte(`
apiVersion: v1
kind: List
items:
  - apiVersion: kyverno.io/v1
    kind: Policy
    metadata:
      name: generate-policy
      namespace: ns-1
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
  - apiVersion: kyverno.io/v1
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
`),
		},
		wantPolicies: []policy{
			{"Policy", "ns-1"},
			{"ClusterPolicy", ""},
		},
		wantErr: false,
	}, {
		name: "ValidatingAdmissionPolicy",
		args: args{
			[]byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "demo-policy.example.com"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   ["apps"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["deployments"]
  validations:
    - expression: "object.spec.replicas <= 5"
`),
		}, vaps: []policy{
			{"ValidatingAdmissionPolicy", ""},
		},
		wantErr: false,
	}, {
		name: "ValidatingAdmissionPolicy and Policy",
		args: args{
			[]byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "demo-policy.example.com"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   ["apps"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["deployments"]
  validations:
    - expression: "object.spec.replicas <= 5"
---
apiVersion: kyverno.io/v1
kind: Policy
metadata:
  name: generate-policy
  namespace: ns-1
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
`),
		}, wantPolicies: []policy{
			{"Policy", "ns-1"},
		},
		vaps: []policy{
			{"ValidatingAdmissionPolicy", ""},
		},
		wantErr: false,
	}, {
		name: "ValidatingAdmissionPolicy and ClusterPolicy",
		args: args{
			[]byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "demo-policy.example.com"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   ["apps"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["deployments"]
  validations:
    - expression: "object.spec.replicas <= 5"
---
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
`),
		}, wantPolicies: []policy{
			{"ClusterPolicy", ""},
		},
		vaps: []policy{
			{"ValidatingAdmissionPolicy", ""},
		},
		wantErr: false,
	}, {
		name: "ValidatingAdmissionPolicyBinding",
		args: args{
			[]byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: "demo-binding-test.example.com"
spec:
  policyName: "demo-policy.example.com"
  validationActions: [Deny]
  matchResources:
    namespaceSelector:
      matchLabels:
        environment: test
`),
		}, vapBindings: []policy{
			{"ValidatingAdmissionPolicyBinding", ""},
		},
		wantErr: false,
	}, {
		name: "ValidatingAdmissionPolicy and its binding",
		args: args{
			[]byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "demo-policy.example.com"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   ["apps"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["deployments"]
  validations:
    - expression: "object.spec.replicas <= 5"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: "demo-binding-test.example.com"
spec:
  policyName: "demo-policy.example.com"
  validationActions: [Deny]
  matchResources:
    namespaceSelector:
      matchLabels:
        environment: test
`),
		}, vaps: []policy{
			{"ValidatingAdmissionPolicy", ""},
		}, vapBindings: []policy{
			{"ValidatingAdmissionPolicyBinding", ""},
		},
		wantErr: false,
	},

		// Mutate Admission Policy
		{
			name: "MutatingAdmissionPolicy",
			args: args{[]byte(`
apiVersion: admissionregistration.k8s.io/v1alpha1
kind: MutatingAdmissionPolicy
metadata:
  name: my-mutation
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["deployments"]
  mutations:
    - patchType: JSONPatch
      jsonPatch:
        expression: "[]"
  reinvocationPolicy: Never
`)},
			MutatingAdmissionPolicies: []policy{{"MutatingAdmissionPolicy", ""}},
			wantErr:                   false,
		},
		// Missing kind must error under strict decoding
		{
			name: "MutatingAdmissionPolicy missing kind",
			args: args{[]byte(`
apiVersion: admissionregistration.k8s.io/v1alpha1
metadata:
  name: missing-kind
`)},
			MutatingAdmissionPolicies: nil,
			wantErr:                   true,
		},
		{
			name:    "MutatingAdmissionPolicy invalid YAML",
			args:    args{[]byte(`: bad yaml`)},
			wantErr: true,
		},
		{
			name: "MutatingAdmissionPolicyBinding",
			args: args{[]byte(`
apiVersion: admissionregistration.k8s.io/v1alpha1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: mapb-demo
spec:
      policyName: my-mutation
      matchResources:
        namespaceSelector:
          matchLabels:
            environment: prod
    `)},
			MutatingAdmissionPolicyBindings: []policy{
				{"MutatingAdmissionPolicyBinding", ""},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPolicies, gotValidatingAdmissionPolicies, gotBindings, _, _, gotMutatingAdmissionPolicies, gotMutatingAdmissionPolicyBinding, err := GetPolicy(tt.args.bytes)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if assert.Equal(t, len(tt.wantPolicies), len(gotPolicies)) {
					for i := range tt.wantPolicies {
						assert.Equal(t, tt.wantPolicies[i].kind, gotPolicies[i].GetKind())
						assert.Equal(t, tt.wantPolicies[i].namespace, gotPolicies[i].GetNamespace())
					}
				}

				if assert.Equal(t, len(tt.vaps), len(gotValidatingAdmissionPolicies)) {
					for i := range tt.vaps {
						assert.Equal(t, tt.vaps[i].kind, gotValidatingAdmissionPolicies[i].Kind)
					}
				}

				if assert.Equal(t, len(tt.vapBindings), len(gotBindings)) {
					for i := range tt.vapBindings {
						assert.Equal(t, tt.vapBindings[i].kind, gotBindings[i].Kind)
					}
				}

				if assert.Equal(t,
					len(tt.MutatingAdmissionPolicies),
					len(gotMutatingAdmissionPolicies),
					"MutatingAdmissionPolicy count",
				) {
					for i := range tt.MutatingAdmissionPolicies {
						assert.Equal(t,
							tt.MutatingAdmissionPolicies[i].kind,
							gotMutatingAdmissionPolicies[i].Kind,
							"MAP[%d].Kind", i,
						)
						assert.Equal(t,
							tt.MutatingAdmissionPolicies[i].namespace,
							gotMutatingAdmissionPolicies[i].GetNamespace(),
							"MAP[%d].Namespace", i,
						)
					}
				}
				if assert.Equal(t,
					len(tt.MutatingAdmissionPolicyBindings),
					len(gotMutatingAdmissionPolicyBinding),
					"MutatingAdmissionPolicyBinding count",
				) {
					for i := range tt.MutatingAdmissionPolicyBindings {
						assert.Equal(t,
							tt.MutatingAdmissionPolicyBindings[i].kind,
							gotMutatingAdmissionPolicyBinding[i].Kind,
							"MAPB[%d].Kind", i,
						)
						assert.Equal(t,
							tt.MutatingAdmissionPolicyBindings[i].namespace,
							gotMutatingAdmissionPolicyBinding[i].GetNamespace(),
							"MAPB[%d].Namespace", i,
						)
					}
				}

			}
		})
	}
}
