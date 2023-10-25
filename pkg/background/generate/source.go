package generate

import (
	"context"

	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func updateSourceLabel(client dclient.Interface, source *unstructured.Unstructured) error {
	labels := source.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	common.TagSource(labels, source)
	source.SetLabels(labels)
	_, err := client.UpdateResource(context.TODO(), source.GetAPIVersion(), source.GetKind(), source.GetNamespace(), source, false)
	return err
}

func addSourceLabels(source *unstructured.Unstructured) {
	labels := source.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 4)
	}

	labels[common.GenerateSourceGroupLabel] = source.GroupVersionKind().Group
	labels[common.GenerateSourceVersionLabel] = source.GroupVersionKind().Version
	labels[common.GenerateSourceKindLabel] = source.GetKind()
	labels[common.GenerateSourceNSLabel] = source.GetNamespace()
	labels[common.GenerateSourceNameLabel] = source.GetName()
	source.SetLabels(labels)
}
