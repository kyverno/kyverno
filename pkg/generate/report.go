package generate

import (
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func reportEvents(err error, eventGen event.Interface, gr kyverno.GenerateRequest, resource unstructured.Unstructured) {
	if err == nil {
		// Success Events
		// - resource -> policy rule applied successfully
		// - policy -> rule successfully applied on resource
		events := successEvents(gr, resource)
		eventGen.Add(events...)
		return
	}
	switch e := err.(type) {
	case *Violation:
		// - resource -> rule failed and created PV
		// - policy -> failed to apply of resource and created PV
		glog.V(4).Infof("reporing events for %v", e)
		events := failedEventsPV(err, gr, resource)
		eventGen.Add(events...)
	default:
		// - resource -> rule failed
		// - policy -> failed tp apply on resource
		glog.V(4).Infof("reporing events for %v", e)
		events := failedEvents(err, gr, resource)
		eventGen.Add(events...)
	}
}

func failedEventsPV(err error, gr kyverno.GenerateRequest, resource unstructured.Unstructured) []event.Info {
	var events []event.Info
	// Cluster Policy
	pe := event.Info{}
	pe.Kind = "ClusterPolicy"
	// cluserwide-resource
	pe.Name = gr.Spec.Policy
	pe.Reason = event.PolicyViolation.String()
	pe.Source = event.GeneratePolicyController
	pe.Message = fmt.Sprintf("policy failed to apply on resource %s/%s/%s creating violation: %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
	events = append(events, pe)

	// Resource
	re := event.Info{}
	re.Kind = resource.GetKind()
	re.Namespace = resource.GetNamespace()
	re.Name = resource.GetName()
	re.Reason = event.PolicyViolation.String()
	re.Source = event.GeneratePolicyController
	re.Message = fmt.Sprintf("policy %s failed to apply created violation: %v", gr.Spec.Policy, err)
	events = append(events, re)

	return events
}

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

func successEvents(gr kyverno.GenerateRequest, resource unstructured.Unstructured) []event.Info {
	var events []event.Info
	// Cluster Policy
	pe := event.Info{}
	pe.Kind = "ClusterPolicy"
	// clusterwide-resource
	pe.Name = gr.Spec.Policy
	pe.Reason = event.PolicyApplied.String()
	pe.Source = event.GeneratePolicyController
	pe.Message = fmt.Sprintf("applied successfully on resource %s/%s/%s", resource.GetKind(), resource.GetNamespace(), resource.GetName())
	events = append(events, pe)

	// Resource
	re := event.Info{}
	re.Kind = resource.GetKind()
	re.Namespace = resource.GetNamespace()
	re.Name = resource.GetName()
	re.Reason = event.PolicyApplied.String()
	re.Source = event.GeneratePolicyController
	re.Message = fmt.Sprintf("policy %s successfully applied", gr.Spec.Policy)
	events = append(events, re)

	return events
}

// buildPathNotPresentPV build violation info when referenced path not found
func buildPathNotPresentPV(er response.EngineResponse) []policyviolation.Info {
	for _, rr := range er.PolicyResponse.Rules {
		if rr.PathNotPresent {
			return policyviolation.GeneratePVsFromEngineResponse([]response.EngineResponse{er})
		}
	}
	return nil
}
