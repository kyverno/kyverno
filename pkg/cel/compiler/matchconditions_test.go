package compiler

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestCompileMatchConditionsWithKubernetesEnv(t *testing.T) {
	tests := []struct {
		name                  string
		conditions            []admissionregistrationv1.MatchCondition
		preexistingExpressions map[string]bool
		wantErrCount          int
		wantErrTypes          []field.ErrorType
	}{{
		name:       "nil conditions",
		conditions: nil,
	}, {
		name:       "empty conditions",
		conditions: []admissionregistrationv1.MatchCondition{},
	}, {
		name: "valid boolean expression",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "check-namespace",
			Expression: `object.metadata.namespace == "default"`,
		}},
	}, {
		name: "valid with authorizer reference",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "authorized-check",
			Expression: `authorizer.group("").resource("pods").check("create").allowed()`,
		}},
	}, {
		name: "multiple valid conditions",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "first",
			Expression: `"foo" == "foo"`,
		}, {
			Name:       "second",
			Expression: `"bar" == "bar"`,
		}},
	}, {
		name: "empty expression",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "empty-expr",
			Expression: "",
		}},
		wantErrCount: 1,
		wantErrTypes: []field.ErrorType{field.ErrorTypeRequired},
	}, {
		name: "whitespace-only expression",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "whitespace-expr",
			Expression: "   ",
		}},
		wantErrCount: 1,
		wantErrTypes: []field.ErrorType{field.ErrorTypeRequired},
	}, {
		name: "invalid CEL expression",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "invalid-expr",
			Expression: `foo()`,
		}},
		wantErrCount: 1,
		wantErrTypes: []field.ErrorType{field.ErrorTypeInvalid},
	}, {
		name: "non-boolean expression",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "non-bool",
			Expression: `"not-a-bool"`,
		}},
		wantErrCount: 1,
		wantErrTypes: []field.ErrorType{field.ErrorTypeInvalid},
	}, {
		name: "empty name",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "",
			Expression: `"foo" == "bar"`,
		}},
		wantErrCount: 1,
		wantErrTypes: []field.ErrorType{field.ErrorTypeRequired},
	}, {
		name: "duplicate condition names",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "duplicate-name",
			Expression: `"foo" == "foo"`,
		}, {
			Name:       "duplicate-name",
			Expression: `"bar" == "bar"`,
		}},
		wantErrCount: 1,
		wantErrTypes: []field.ErrorType{field.ErrorTypeDuplicate},
	}, {
		name: "too many conditions",
		conditions: func() []admissionregistrationv1.MatchCondition {
			conditions := make([]admissionregistrationv1.MatchCondition, 65)
			for i := range conditions {
				conditions[i] = admissionregistrationv1.MatchCondition{
					Name:       fmt.Sprintf("condition-%d", i),
					Expression: `true`,
				}
			}
			return conditions
		}(),
		wantErrCount: 1,
		wantErrTypes: []field.ErrorType{field.ErrorTypeTooMany},
	}, {
		name: "preexisting expression uses stored env",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "preexisting",
			Expression: `"foo" == "bar"`,
		}},
		preexistingExpressions: map[string]bool{`"foo" == "bar"`: true},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := CompileMatchConditionsWithKubernetesEnv(tt.conditions, tt.preexistingExpressions)
			if tt.wantErrCount == 0 {
				assert.Empty(t, errs)
			} else {
				assert.Len(t, errs, tt.wantErrCount)
				for i, errType := range tt.wantErrTypes {
					if i < len(errs) {
						assert.Equal(t, errType, errs[i].Type)
					}
				}
			}
		})
	}
}
