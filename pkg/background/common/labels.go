package common

import (
	"fmt"
	"reflect"
	"strings"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	pkglabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Object interface {
	GetName() string
	GetNamespace() string
	GetKind() string
	GetAPIVersion() string
}

func ManageLabels(unstr *unstructured.Unstructured, triggerResource unstructured.Unstructured) {
	// add managedBY label if not defined
	labels := unstr.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	// handle managedBy label
	managedBy(labels)
	// handle generatedBy label
	generatedBy(labels, triggerResource)

	// update the labels
	unstr.SetLabels(labels)
}

func MutateLabelsSet(policyKey string, trigger Object) pkglabels.Set {
	_, policyName, _ := cache.SplitMetaNamespaceKey(policyKey)

	set := pkglabels.Set{
		kyvernov1beta1.URMutatePolicyLabel: policyName,
	}
	isNil := trigger == nil || (reflect.ValueOf(trigger).Kind() == reflect.Ptr && reflect.ValueOf(trigger).IsNil())
	if !isNil {
		set[kyvernov1beta1.URMutateTriggerNameLabel] = trigger.GetName()
		set[kyvernov1beta1.URMutateTriggerNSLabel] = trigger.GetNamespace()
		set[kyvernov1beta1.URMutatetriggerKindLabel] = trigger.GetKind()
		if trigger.GetAPIVersion() != "" {
			set[kyvernov1beta1.URMutatetriggerAPIVersionLabel] = strings.ReplaceAll(trigger.GetAPIVersion(), "/", "-")
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
		set[kyvernov1beta1.URGenerateResourceNameLabel] = trigger.GetName()
		set[kyvernov1beta1.URGenerateResourceNSLabel] = trigger.GetNamespace()
		set[kyvernov1beta1.URGenerateResourceKindLabel] = trigger.GetKind()
	}
	return set
}

func managedBy(labels map[string]string) {
	// ManagedBy label
	key := "app.kubernetes.io/managed-by"
	value := "kyverno"
	val, ok := labels[key]
	if ok {
		if val != value {
			log.Log.Info(fmt.Sprintf("resource managed by %s, kyverno wont over-ride the label", val))
			return
		}
	}
	if !ok {
		// add label
		labels[key] = value
	}
}

func generatedBy(labels map[string]string, triggerResource unstructured.Unstructured) {
	keyKind := "kyverno.io/generated-by-kind"
	keyNamespace := "kyverno.io/generated-by-namespace"
	keyName := "kyverno.io/generated-by-name"

	checkGeneratedBy(labels, keyKind, triggerResource.GetKind())
	checkGeneratedBy(labels, keyNamespace, triggerResource.GetNamespace())
	checkGeneratedBy(labels, keyName, triggerResource.GetName())
}

func checkGeneratedBy(labels map[string]string, key, value string) {
	if len(value) > 63 {
		value = value[0:63]
	}

	val, ok := labels[key]
	if ok {
		if val != value {
			log.Log.Info(fmt.Sprintf("kyverno wont over-ride the label %s", key))
			return
		}
	}
	if !ok {
		// add label
		labels[key] = value
	}
}
