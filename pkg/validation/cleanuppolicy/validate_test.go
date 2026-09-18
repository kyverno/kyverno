package cleanuppolicy

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAllowedVariables_Anchored(t *testing.T) {
	tcs := []struct {
		name    string
		key     string
		context []kyvernov1.ContextEntry
		wantErr bool
	}{
		{
			name: "allow_target",
			key:  "{{ target.metadata.name }}",
		},
		{
			name: "allow_images",
			key:  "{{ images.containers.nginx }}",
		},
		{
			name: "allow_jmespath_func",
			key:  "{{ length(target.metadata.name) }}",
		},
		{
			name: "allow_context_var",
			key:  "{{ varname }}",
			context: []kyvernov1.ContextEntry{{
				Name: "varname",
				Variable: &kyvernov1.Variable{
					Value: kyverno.ToAny("example"),
				},
			}},
		},
		{
			name:    "reject_request_userInfo",
			key:     "{{ request.userInfo.username }}",
			wantErr: true,
		},
		{
			name:    "reject_request_object",
			key:     "{{ request.object.metadata.name }}",
			wantErr: true,
		},
		{
			name:    "reject_substring_target",
			key:     "{{ mytarget.metadata.name }}",
			wantErr: true,
		},
		{
			name:    "reject_alphanumeric_garbage",
			key:     "{{ totally_invalid_var }}",
			wantErr: true,
		},
		{
			name:    "reject_element_substring",
			key:     "{{ elementss }}",
			wantErr: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			policy := &kyvernov2.ClusterCleanupPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: kyvernov2.CleanupPolicySpec{
					Context: tc.context,
					Conditions: &kyvernov2.AnyAllConditions{
						AllConditions: []kyvernov2.Condition{{
							RawKey:   kyverno.ToAny(tc.key),
							Operator: kyvernov2.ConditionOperators["Equals"],
							RawValue: kyverno.ToAny("x"),
						}},
					},
				},
			}
			err := validateVariables(logr.Discard(), policy)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
