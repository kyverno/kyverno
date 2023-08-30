package generate

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newResourceSpec(genAPIVersion, genKind, genNamespace, genName string) kyvernov1.ResourceSpec {
	return kyvernov1.ResourceSpec{
		APIVersion: genAPIVersion,
		Kind:       genKind,
		Namespace:  genNamespace,
		Name:       genName,
	}
}

func TriggerFromLabels(labels map[string]string) kyvernov1.ResourceSpec {
	group := labels[common.GenerateTriggerGroupLabel]
	version := labels[common.GenerateTriggerVersionLabel]
	apiVersion := schema.GroupVersion{Group: group, Version: version}

	return kyvernov1.ResourceSpec{
		Kind:       labels[common.GenerateTriggerKindLabel],
		Namespace:  labels[common.GenerateTriggerNSLabel],
		Name:       labels[common.GenerateTriggerNameLabel],
		APIVersion: apiVersion.String(),
	}
}

func FindDownstream(client dclient.Interface, apiVersion, kind string, labels map[string]string) (*unstructured.UnstructuredList, error) {
	selector := &metav1.LabelSelector{MatchLabels: labels}
	return client.ListResource(context.TODO(), apiVersion, kind, "", selector)
}
