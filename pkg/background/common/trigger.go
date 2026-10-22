package common

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// TriggerFromLabels rebuilds the trigger ResourceSpec from the labels a
// generated downstream resource carries (set by TriggerInfo/PolicyInfo).
func TriggerFromLabels(labels map[string]string) kyvernov1.ResourceSpec {
	group := labels[GenerateTriggerGroupLabel]
	version := labels[GenerateTriggerVersionLabel]
	apiVersion := schema.GroupVersion{Group: group, Version: version}

	return kyvernov1.ResourceSpec{
		Kind:       labels[GenerateTriggerKindLabel],
		Namespace:  labels[GenerateTriggerNSLabel],
		Name:       labels[GenerateTriggerNameLabel],
		UID:        types.UID(labels[GenerateTriggerUIDLabel]),
		APIVersion: apiVersion.String(),
	}
}
