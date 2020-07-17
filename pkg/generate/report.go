package generate

import (
	"fmt"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/event"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func failedEvents(err error, gr kyverno.GenerateRequest, resource unstructured.Unstructured) []event.Info {
	var events []event.Info
	// Cluster Policy
	pe := event.Info{}
	pe.Kind = "ClusterPolicy"
	// cluserwide-resource
	pe.Name = gr.Spec.Policy
	pe.Reason = event.PolicyFailed.String()
	pe.Source = event.GeneratePolicyController
	pe.Message = fmt.Sprintf("policy failed to apply on resource %s/%s/%s: %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
	events = append(events, pe)

	// Resource
	re := event.Info{}
	re.Kind = resource.GetKind()
	re.Namespace = resource.GetNamespace()
	re.Name = resource.GetName()
	re.Reason = event.PolicyFailed.String()
	re.Source = event.GeneratePolicyController
	re.Message = fmt.Sprintf("policy %s failed to apply: %v", gr.Spec.Policy, err)
	events = append(events, re)

	return events
}
