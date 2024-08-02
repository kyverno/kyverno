package common

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/logging"
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
		kyvernov1beta1.URMutatePolicyLabel: policyName,
	}
	isNil := trigger == nil || (reflect.ValueOf(trigger).Kind() == reflect.Ptr && reflect.ValueOf(trigger).IsNil())
	if !isNil {
		set[kyvernov1beta1.URMutateTriggerNameLabel] = trimByLength(trigger.GetName(), 63)
		set[kyvernov1beta1.URMutateTriggerNSLabel] = trigger.GetNamespace()
		set[kyvernov1beta1.URMutateTriggerKindLabel] = trigger.GetKind()
		if trigger.GetAPIVersion() != "" {
			set[kyvernov1beta1.URMutateTriggerAPIVersionLabel] = strings.ReplaceAll(trigger.GetAPIVersion(), "/", "-")
		}
	}
	return set
}

func GenerateLabelsSet(policyKey string, trigger Object) pkglabels.Set {
	_, policyName, _ := cache.SplitMetaNamespaceKey(policyKey)

	set := pkglabels.Set{
		kyvernov1beta1.URGeneratePolicyLabel: policyName,
	}
	isNil := trigger == nil || (reflect.ValueOf(trigger).Kind() == reflect.Ptr && reflect.ValueOf(trigger).IsNil())
	if !isNil {
		set[kyvernov1beta1.URGenerateResourceUIDLabel] = string(trigger.GetUID())
		set[kyvernov1beta1.URGenerateResourceNSLabel] = trigger.GetNamespace()
		set[kyvernov1beta1.URGenerateResourceKindLabel] = trigger.GetKind()
	}
	return set
}

func managedBy(labels map[string]string) {
	// ManagedBy label
	key := kyverno.LabelAppManagedBy
	value := kyverno.ValueKyvernoApp
	val, ok := labels[key]
	if ok {
		if val != value {
			logging.V(2).Info(fmt.Sprintf("resource managed by %s, kyverno wont over-ride the label", val))
			return
		}
	}
	if !ok {
		// add label
		labels[key] = value
	}
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
