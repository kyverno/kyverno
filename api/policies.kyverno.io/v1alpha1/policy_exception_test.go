package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestCELPolicyException_GetKind(t *testing.T) {
	tests := []struct {
		name   string
		policy *PolicyException
		want   string
	}{{
		name:   "not set",
		policy: &PolicyException{},
		want:   "PolicyException",
	}, {
		name: "not set",
		policy: &PolicyException{
			TypeMeta: v1.TypeMeta{
				Kind: "Foo",
			},
		},
		want: "PolicyException",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetKind()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCELPolicyExceptionSpec_Validate(t *testing.T) {
	tests := []struct {
		name     string
		policy   *PolicyException
		wantErrs field.ErrorList
	}{{
		name:   "no refs",
		policy: &PolicyException{},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "spec.policyRefs",
			BadValue: []PolicyRef(nil),
			Detail:   "must specify at least one policy ref",
		}},
	}, {
		name: "one ref",
		policy: &PolicyException{
			Spec: PolicyExceptionSpec{
				PolicyRefs: []PolicyRef{{
					Name: "foo",
					Kind: "Foo",
				}},
			},
		},
		wantErrs: nil,
	}, {
		name: "ref no kind",
		policy: &PolicyException{
			Spec: PolicyExceptionSpec{
				PolicyRefs: []PolicyRef{{
					Name: "foo",
				}},
			},
		},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "spec.policyRefs[0].kind",
			BadValue: "",
			Detail:   "must specify policy kind",
		}},
	}, {
		name: "ref no name",
		policy: &PolicyException{
			Spec: PolicyExceptionSpec{
				PolicyRefs: []PolicyRef{{
					Kind: "Foo",
				}},
			},
		},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "spec.policyRefs[0].name",
			BadValue: "",
			Detail:   "must specify policy name",
		}},
	},
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErrs := tt.policy.Validate()
			assert.Equal(t, tt.wantErrs, gotErrs)
		})
	}
}
