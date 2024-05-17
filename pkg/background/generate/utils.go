package generate

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
		UID:        types.UID(labels[common.GenerateTriggerUIDLabel]),
		APIVersion: apiVersion.String(),
	}
}
