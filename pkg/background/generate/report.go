package generate

import (
	"fmt"

	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/event"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func failedEvents(err error, gr urkyverno.UpdateRequest, resource unstructured.Unstructured) []event.Info {
	re := event.Info{}
	re.Kind = resource.GetKind()
	re.Namespace = resource.GetNamespace()
	re.Name = resource.GetName()
	re.Reason = event.PolicyFailed.String()
	re.Source = event.GeneratePolicyController
	re.Message = fmt.Sprintf("policy %s failed to apply: %v", gr.Spec.Policy, err)

	return []event.Info{re}
}
