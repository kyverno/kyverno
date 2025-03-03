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
		policy *CELPolicyException
		want   string
	}{{
		name:   "not set",
		policy: &CELPolicyException{},
		want:   "CELPolicyException",
	}, {
		name: "not set",
		policy: &CELPolicyException{
			TypeMeta: v1.TypeMeta{
				Kind: "Foo",
			},
		},
		want: "CELPolicyException",
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
		policy   *CELPolicyException
		wantErrs field.ErrorList
	}{{
		name:   "no refs",
		policy: &CELPolicyException{},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "spec.policyRefs",
			BadValue: []PolicyRef(nil),
			Detail:   "must specify at least one policy ref",
		}},
	}, {
		name: "one ref",
		policy: &CELPolicyException{
			Spec: CELPolicyExceptionSpec{
				PolicyRefs: []PolicyRef{{
					Name: "foo",
					Kind: "Foo",
				}},
			},
		},
		wantErrs: nil,
	}, {
		name: "ref no kind",
		policy: &CELPolicyException{
			Spec: CELPolicyExceptionSpec{
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
		policy: &CELPolicyException{
			Spec: CELPolicyExceptionSpec{
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
