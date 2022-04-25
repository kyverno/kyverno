package common

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/event"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var EmptyPatchErr error = fmt.Errorf("empty resource to patch")

func FailedEvents(err error, policy, rule string, source event.Source, resource *unstructured.Unstructured, logger logr.Logger) []event.Info {
	if resource == nil {
		logger.WithName("FailedEvents").Error(EmptyPatchErr, "", "rule", rule)
		return nil
	}

	re := newEvent(policy, rule, source, *resource)

	re.Reason = event.PolicyFailed.String()
	re.Message = fmt.Sprintf("policy %s/%s failed to apply to %s/%s/%s: %v", policy, rule, resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)

	return []event.Info{re}
}

func SucceedEvents(policy, rule string, source event.Source, resource *unstructured.Unstructured, logger logr.Logger) []event.Info {
	if resource == nil {
		logger.WithName("SucceedEvents").Error(EmptyPatchErr, "", "rule", rule)
		return nil
	}

	re := newEvent(policy, rule, source, *resource)

	re.Reason = event.PolicyApplied.String()
	re.Message = fmt.Sprintf("policy %s/%s applied to %s/%s/%s successfully", policy, rule, resource.GetKind(), resource.GetNamespace(), resource.GetName())

	return []event.Info{re}
}

func newEvent(policy, rule string, source event.Source, resource unstructured.Unstructured) (re event.Info) {
	re.Kind = resource.GetKind()
	re.Namespace = resource.GetNamespace()
	re.Name = resource.GetName()
	re.Source = source
	return
}
