package compiler

// import (
// 	"testing"

// 	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
// 	"github.com/stretchr/testify/assert"
// 	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// )

// func Test_compiler_Compile(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		policy  *policiesv1alpha1.ValidatingPolicy
// 		wantErr bool
// 	}{{
// 		name: "simple",
// 		policy: &policiesv1alpha1.ValidatingPolicy{
// 			TypeMeta: metav1.TypeMeta{
// 				APIVersion: policiesv1alpha1.GroupVersion.String(),
// 				Kind:       "ValidatingPolicy",
// 			},
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name: "foo",
// 			},
// 			Spec: policiesv1alpha1.ValidatingPolicySpec{
// 				MatchConstraints: &admissionregistrationv1.MatchResources{
// 					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
// 						{
// 							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
// 								Operations: []admissionregistrationv1.OperationType{
// 									admissionregistrationv1.Create,
// 									admissionregistrationv1.Update,
// 								},
// 								Rule: admissionregistrationv1.Rule{
// 									APIGroups:   []string{""},
// 									APIVersions: []string{"v1"},
// 									Resources:   []string{"pods"},
// 								},
// 							},
// 						},
// 					},
// 				},
// 				Variables: []admissionregistrationv1.Variable{{
// 					Name:       "environment",
// 					Expression: "has(object.metadata.labels) && 'env' in object.metadata.labels && object.metadata.labels['env'] == 'prod'",
// 				}},
// 				Validations: []admissionregistrationv1.Validation{{
// 					Expression: "variables.environment == true",
// 				}},
// 			},
// 		},
// 	}, {
// 		name: "with configmap",
// 		policy: &policiesv1alpha1.ValidatingPolicy{
// 			TypeMeta: metav1.TypeMeta{
// 				APIVersion: policiesv1alpha1.GroupVersion.String(),
// 				Kind:       "ValidatingPolicy",
// 			},
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name: "foo",
// 			},
// 			Spec: policiesv1alpha1.ValidatingPolicySpec{
// 				MatchConstraints: &admissionregistrationv1.MatchResources{
// 					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
// 						{
// 							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
// 								Operations: []admissionregistrationv1.OperationType{
// 									admissionregistrationv1.Create,
// 									admissionregistrationv1.Update,
// 								},
// 								Rule: admissionregistrationv1.Rule{
// 									APIGroups:   []string{""},
// 									APIVersions: []string{"v1"},
// 									Resources:   []string{"pods"},
// 								},
// 							},
// 						},
// 					},
// 				},
// 				Variables: []admissionregistrationv1.Variable{{
// 					Name:       "cm",
// 					Expression: "resource.Get('v1', 'configmaps', 'foo', 'bar')",
// 				}},
// 				Validations: []admissionregistrationv1.Validation{{
// 					Expression: "variables.cm != null",
// 				}},
// 			},
// 		},
// 	}, {
// 		name: "post resource",
// 		policy: &policiesv1alpha1.ValidatingPolicy{
// 			TypeMeta: metav1.TypeMeta{
// 				APIVersion: policiesv1alpha1.GroupVersion.String(),
// 				Kind:       "ValidatingPolicy",
// 			},
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name: "foo",
// 			},
// 			Spec: policiesv1alpha1.ValidatingPolicySpec{
// 				MatchConstraints: &admissionregistrationv1.MatchResources{
// 					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
// 						{
// 							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
// 								Operations: []admissionregistrationv1.OperationType{
// 									admissionregistrationv1.Create,
// 									admissionregistrationv1.Update,
// 								},
// 								Rule: admissionregistrationv1.Rule{
// 									APIGroups:   []string{""},
// 									APIVersions: []string{"v1"},
// 									Resources:   []string{"pods"},
// 								},
// 							},
// 						},
// 					},
// 				},
// 				Variables: []admissionregistrationv1.Variable{{
// 					Name:       "cm",
// 					Expression: "resource.Post('authorization.k8s.io/v1', 'subjectaccessreviews', {})",
// 				}},
// 				Validations: []admissionregistrationv1.Validation{{
// 					Expression: "variables.cm != null",
// 				}},
// 			},
// 		},
// 	}}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			c := NewCompiler()
// 			compiled, errs := c.CompileValidating(tt.policy, nil)
// 			if tt.wantErr {
// 				assert.Error(t, errs.ToAggregate())
// 			} else {
// 				assert.NoError(t, errs.ToAggregate())
// 				assert.NotNil(t, compiled)
// 			}
// 		})
// 	}
// }
