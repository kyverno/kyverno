package policy

import (
	"reflect"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
)

func TestExtractVariables(t *testing.T) {
	loadPolicy := func(path string) kyvernov1.PolicyInterface {
		t.Helper()
		policies, _, err := Load(nil, "", path)
		assert.NoError(t, err)
		assert.Equal(t, len(policies), 1)
		return policies[0]

	}
	tests := []struct {
		name    string
		policy  kyvernov1.PolicyInterface
		want    []string
		wantErr bool
	}{{
		name:    "nil",
		policy:  nil,
		want:    nil,
		wantErr: false,
	}, {
		name:    "cpol-pod-requirements",
		policy:  loadPolicy("../_testdata/policies/cpol-pod-requirements.yaml"),
		want:    nil,
		wantErr: false,
	}, {
		name:   "cpol-limit-configmap-for-sa",
		policy: loadPolicy("../_testdata/policies/cpol-limit-configmap-for-sa.yaml"),
		want: []string{
			"{{request.object.metadata.namespace}}",
			"{{request.object.metadata.name}}",
			"{{request.object.metadata.namespace}}",
			"{{request.object.kind}}",
			"{{request.object.metadata.name}}",
			"{{request.operation}}",
		},
		wantErr: false,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractVariables(tt.policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}
