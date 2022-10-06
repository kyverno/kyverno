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
		name         string
		args         args
		wantPolicies []policy
		wantErr      bool
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
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPolicies, err := GetPolicy(tt.args.bytes)
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
			}
		})
	}
}
