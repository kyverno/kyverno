package test

import (
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
)

func Test_loadTest(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *api.Test
		wantErr bool
	}{{
		name: "invalid schema",
		data: []byte(`
- name: mytest
  policies:
    - pol.yaml
  resources:
    - pod.yaml
  results:
  - policy: evil-policy-match-foreign-pods
    rule: evil-validation
    resource: nginx
    status: pass
`),
		want:    nil,
		wantErr: true,
	}, {
		name: "unknown field",
		data: []byte(`
name: mytest
policies:
  - pol.yaml
resources:
  - pod.yaml
results:
- policy: evil-policy-match-foreign-pods
  rule: evil-validation
  resource: nginx
  foo: bar
  result: pass
`),
		want:    nil,
		wantErr: true,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadTest(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadTest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadTest() = %v, want %v", got, tt.want)
			}
		})
	}
}
