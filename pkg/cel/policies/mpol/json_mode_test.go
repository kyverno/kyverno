package mpol

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func jsonModePolicy(expression string) *v1beta1.MutatingPolicy {
	return &v1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "json"},
		Spec: v1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &v1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: expression},
			}},
		},
	}
}

// JSON mode policies are admitted by policy validation without the Kubernetes
// admission prerequisites (matchConstraints, admission or mutateExisting enabled),
// while Kubernetes mode keeps requiring them.
func TestValidate_JSONMode(t *testing.T) {
	tests := []struct {
		name    string
		pol     v1beta1.MutatingPolicyLike
		wantErr string
	}{
		{
			name: "json policy without matchConstraints is valid",
			pol:  jsonModePolicy(`[JSONPatch{op:"add",path:"/a",value:1}]`),
		},
		{
			name: "json namespaced policy without matchConstraints is valid",
			pol: &v1beta1.NamespacedMutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "json", Namespace: "ns"},
				Spec:       jsonModePolicy(`[JSONPatch{op:"add",path:"/a",value:1}]`).Spec,
			},
		},
		{
			name: "json policy with admission and background disabled is valid",
			pol: func() *v1beta1.MutatingPolicy {
				p := jsonModePolicy(`[JSONPatch{op:"add",path:"/a",value:1}]`)
				p.Spec.EvaluationConfiguration.Admission = &v1beta1.AdmissionConfiguration{Enabled: ptr.To(false)}
				p.Spec.EvaluationConfiguration.Background = &v1beta1.BackgroundConfiguration{Enabled: ptr.To(false)}
				p.Spec.EvaluationConfiguration.MutateExistingConfiguration = &v1beta1.MutateExistingConfiguration{Enabled: ptr.To(false)}
				return p
			}(),
		},
		{
			name: "json policy with empty autogen is valid",
			pol: func() *v1beta1.MutatingPolicy {
				p := jsonModePolicy(`[JSONPatch{op:"add",path:"/a",value:1}]`)
				p.Spec.AutogenConfiguration = &v1beta1.MutatingPolicyAutogenConfiguration{}
				return p
			}(),
		},
		{
			name: "json policy with pod controller autogen is rejected",
			pol: func() *v1beta1.MutatingPolicy {
				p := jsonModePolicy(`[JSONPatch{op:"add",path:"/a",value:1}]`)
				p.Spec.AutogenConfiguration = &v1beta1.MutatingPolicyAutogenConfiguration{
					PodControllers: &v1beta1.PodControllersGenerationConfiguration{Controllers: []string{"deployments"}},
				}
				return p
			}(),
			wantErr: "autogen.podControllers.controllers",
		},
		{
			name: "json policy with mutateExisting is rejected",
			pol: func() *v1beta1.MutatingPolicy {
				p := jsonModePolicy(`[JSONPatch{op:"add",path:"/a",value:1}]`)
				p.Spec.EvaluationConfiguration.MutateExistingConfiguration = &v1beta1.MutateExistingConfiguration{Enabled: ptr.To(true)}
				return p
			}(),
			wantErr: "evaluation.mutateExisting",
		},
		{
			name:    "json policy with kubernetes-only expression is rejected",
			pol:     jsonModePolicy(`[JSONPatch{op:"add",path:"/a",value:namespace.metadata.name}]`),
			wantErr: "mutations[0]",
		},
		{
			name: "json policy with applyConfiguration is rejected",
			pol: func() *v1beta1.MutatingPolicy {
				p := jsonModePolicy("")
				p.Spec.Mutations = []admissionregistrationv1alpha1.Mutation{{
					PatchType:          admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `Object{}`},
				}}
				return p
			}(),
			wantErr: "only JSONPatch mutations are supported",
		},
		{
			name: "kubernetes policy still requires matchConstraints",
			pol: &v1beta1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "k8s"},
				Spec: v1beta1.MutatingPolicySpec{
					EvaluationConfiguration: &v1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeKubernetes},
				},
			},
			wantErr: "matchConstraints",
		},
		{
			name: "kubernetes policy with matchConstraints is valid",
			pol: &v1beta1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "k8s"},
				Spec: v1beta1.MutatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{APIGroups: []string{"apps"}, Resources: []string{"deployments"}},
							},
						}},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := Validate(tt.pol)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Empty(t, warnings)
		})
	}
}
