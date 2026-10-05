package mpol

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// An UpdateRequest that points at a JSON mode policy must be failed terminally
// instead of retrying forever as "not compiled yet" or touching the engine.
func TestProcess_JSONModePolicyFailsTerminally(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset(&policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "json"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		},
	})
	engine := &fakeEngine{}
	status := &fakeStatusControl{}
	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		kyvernoClient,
		engine,
		meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}}),
		&libs.FakeContextProvider{},
		status,
		event.NewFake(),
		nil,
	)
	err := p.Process(&kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-json", Namespace: "default"},
		Spec:       kyvernov2.UpdateRequestSpec{Policy: "json"},
	})
	assert.NoError(t, err)
	assert.True(t, status.failedCalled)
	assert.False(t, status.successCalled)
	engine.AssertNotCalled(t, "Evaluate")
}
