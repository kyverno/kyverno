package exception

import (
	"testing"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_compiler_Compile(t *testing.T) {
	tests := []struct {
		name      string
		exception *kyvernov2alpha1.CELPolicyException
		wantErr   bool
	}{
		{
			name: "use object",
			exception: &kyvernov2alpha1.CELPolicyException{
				TypeMeta: metav1.TypeMeta{
					APIVersion: kyvernov2alpha1.GroupVersion.String(),
					Kind:       "CELPolicyException",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "exception",
				},
				Spec: kyvernov2alpha1.CELPolicyExceptionSpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "check env label",
							Expression: "has(object.metadata.labels) && 'env' in object.metadata.labels && object.metadata.labels['env'] == 'prod'",
						},
					},
				},
			},
		},
		{
			name: "use namespaceObject",
			exception: &kyvernov2alpha1.CELPolicyException{
				TypeMeta: metav1.TypeMeta{
					APIVersion: kyvernov2alpha1.GroupVersion.String(),
					Kind:       "CELPolicyException",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "exception",
				},
				Spec: kyvernov2alpha1.CELPolicyExceptionSpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "check namespace name",
							Expression: "namespaceObject.metadata.name != 'default'",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCompiler()
			compiled, errs := c.Compile(tt.exception)
			if tt.wantErr {
				assert.Error(t, errs.ToAggregate())
			} else {
				assert.NoError(t, errs.ToAggregate())
				assert.NotNil(t, compiled)
			}
		})
	}
}
