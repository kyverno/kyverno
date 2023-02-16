package generate

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func updateSourceLabel(client dclient.Interface, source *unstructured.Unstructured, trigger kyvernov1.ResourceSpec, policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) error {
	labels := source.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	common.PolicyInfo(labels, policy, rule.Name)
	common.TriggerInfo(labels, trigger)

	source.SetLabels(labels)
	_, err := client.UpdateResource(context.TODO(), source.GetAPIVersion(), source.GetKind(), source.GetNamespace(), source, false)
	return err
}
