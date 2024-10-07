package common

import (
	"reflect"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	pkglabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type Object interface {
	GetName() string
	GetNamespace() string
	GetKind() string
	GetAPIVersion() string
	GetUID() types.UID
}

func ManageLabels(unstr *unstructured.Unstructured, triggerResource unstructured.Unstructured, policy kyvernov1.PolicyInterface, ruleName string) {
	labels := unstr.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	managedBy(labels)
	PolicyInfo(labels, policy, ruleName)
	TriggerInfo(labels, triggerResource)
	unstr.SetLabels(labels)
}

func MutateLabelsSet(policyKey string, trigger Object) pkglabels.Set {
	_, policyName, _ := cache.SplitMetaNamespaceKey(policyKey)

	set := pkglabels.Set{
		kyvernov2.URMutatePolicyLabel: policyName,
	}
	isNil := trigger == nil || (reflect.ValueOf(trigger).Kind() == reflect.Ptr && reflect.ValueOf(trigger).IsNil())
	if !isNil {
		set[kyvernov2.URMutateTriggerNameLabel] = trimByLength(trigger.GetName(), 63)
		set[kyvernov2.URMutateTriggerNSLabel] = trigger.GetNamespace()
		set[kyvernov2.URMutateTriggerKindLabel] = trigger.GetKind()
		if trigger.GetAPIVersion() != "" {
			set[kyvernov2.URMutateTriggerAPIVersionLabel] = strings.ReplaceAll(trigger.GetAPIVersion(), "/", "-")
		}
	}
	return set
}

func GenerateLabelsSet(policyKey string) pkglabels.Set {
	_, policyName, _ := cache.SplitMetaNamespaceKey(policyKey)

	set := pkglabels.Set{
		kyvernov2.URGeneratePolicyLabel: policyName,
	}
	return set
}

func managedBy(labels map[string]string) {
	labels[kyverno.LabelAppManagedBy] = kyverno.ValueKyvernoApp
}

func PolicyInfo(labels map[string]string, policy kyvernov1.PolicyInterface, ruleName string) {
	labels[GeneratePolicyLabel] = policy.GetName()
	labels[GeneratePolicyNamespaceLabel] = policy.GetNamespace()
	labels[GenerateRuleLabel] = ruleName
}

func TriggerInfo(labels map[string]string, obj unstructured.Unstructured) {
	labels[GenerateTriggerVersionLabel] = obj.GroupVersionKind().Version
	labels[GenerateTriggerGroupLabel] = obj.GroupVersionKind().Group
	labels[GenerateTriggerKindLabel] = obj.GetKind()
	labels[GenerateTriggerNSLabel] = obj.GetNamespace()
	labels[GenerateTriggerUIDLabel] = string(obj.GetUID())
}

func TagSource(labels map[string]string, obj Object) {
	labels[GenerateTypeCloneSourceLabel] = ""
}

func trimByLength(value string, character int) string {
	if len(value) > character {
		return value[0:character]
	}
	return value
}
