package generate

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func manageLabels(unstr *unstructured.Unstructured, triggerResource unstructured.Unstructured) {
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
