package common

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	enginutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/informers"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Policy Reporting Modes
const (
	// Enforce blocks the request on failure
	Enforce = "enforce"
	// Audit indicates not to block the request on failure, but report failiures as policy violations
	Audit = "audit"
)

// Policy Reporting Types
const (
	PolicyViolation = "POLICYVIOLATION"
	PolicyReport    = "POLICYREPORT"
)

// GetNamespaceSelectorsFromGenericInformer - extracting the namespacelabels when generic informer is passed
func GetNamespaceSelectorsFromGenericInformer(kind, namespaceOfResource string, nsInformer informers.GenericInformer, logger logr.Logger) map[string]string {
	namespaceLabels := make(map[string]string)
	if kind != "Namespace" {
		runtimeNamespaceObj, err := nsInformer.Lister().Get(namespaceOfResource)
		if err != nil {
			log.Log.Error(err, "failed to get the namespace", "name", namespaceOfResource)
			return namespaceLabels
		}

		unstructuredObj := runtimeNamespaceObj.(*unstructured.Unstructured)
		return unstructuredObj.GetLabels()
	}
	return namespaceLabels
}

// GetNamespaceSelectorsFromNamespaceLister - extract the namespacelabels when namespace lister is passed
func GetNamespaceSelectorsFromNamespaceLister(kind, namespaceOfResource string, nsLister listerv1.NamespaceLister, logger logr.Logger) map[string]string {
	namespaceLabels := make(map[string]string)
	if kind != "Namespace" {
		namespaceObj, err := nsLister.Get(namespaceOfResource)
		if err != nil {
			log.Log.Error(err, "failed to get the namespace", "name", namespaceOfResource)
			return namespaceLabels
		}
		return GetNamespaceLabels(namespaceObj, logger)
	}
	return namespaceLabels
}

// GetNamespaceLabels - from namespace obj
func GetNamespaceLabels(namespaceObj *v1.Namespace, logger logr.Logger) map[string]string {
	namespaceObj.Kind = "Namespace"
	namespaceRaw, err := json.Marshal(namespaceObj)
	if err != nil {
		logger.Error(err, "failed to marshal namespace")
	}
	namespaceUnstructured, err := enginutils.ConvertToUnstructured(namespaceRaw)
	if err != nil {
		logger.Error(err, "failed to convert object resource to unstructured format")
	}
	return namespaceUnstructured.GetLabels()
}

// GetKindFromGVK - get kind and APIVersion from GVK
func GetKindFromGVK(str string) (apiVersion string, kind string) {
	if strings.Count(str, "/") == 0 {
		return "", str
	}
	splitString := strings.Split(str, "/")
	if strings.Count(str, "/") == 1 {
		return splitString[0], splitString[1]
	}
	return splitString[0] + "/" + splitString[1], splitString[2]
}

func VariableToJSON(key, value string) []byte {
	var subString string
	splitBySlash := strings.Split(key, "\"")
	if len(splitBySlash) > 1 {
		subString = splitBySlash[1]
	}

	startString := ""
	endString := ""
	lenOfVariableString := 0
	addedSlashString := false
	for _, k := range strings.Split(splitBySlash[0], ".") {
		if k != "" {
			startString += fmt.Sprintf(`{"%s":`, k)
			endString += `}`
			lenOfVariableString = lenOfVariableString + len(k) + 1
			if lenOfVariableString >= len(splitBySlash[0]) && len(splitBySlash) > 1 && !addedSlashString {
				startString += fmt.Sprintf(`{"%s":`, subString)
				endString += `}`
				addedSlashString = true
			}
		}
	}

	midString := fmt.Sprintf(`"%s"`, strings.Replace(value, `"`, `\"`, -1))
	finalString := startString + midString + endString
	var jsonData = []byte(finalString)
	return jsonData
}

// RetryFunc allows retrying a function on error within a given timeout
func RetryFunc(retryInterval, timeout time.Duration, run func() error, logger logr.Logger) func() error {
	return func() error {
		registerTimeout := time.After(timeout)
		registerTicker := time.NewTicker(retryInterval)
		defer registerTicker.Stop()
		var err error

	loop:
		for {
			select {
			case <-registerTicker.C:
				err = run()
				if err != nil {
					logger.V(3).Info("Failed to register admission control webhooks", "reason", err.Error())
				} else {
					break loop
				}

			case <-registerTimeout:
				return errors.Wrap(err, "Timeout registering admission control webhooks")
			}
		}
		return nil
	}
}

func ProcessDeletePolicyForCloneGenerateRule(rules []kyverno.Rule, client *dclient.Client, pName string, logger logr.Logger) bool {
	generatePolicyWithClone := false
	for _, rule := range rules {
		if rule.Generation.Clone.Name == "" {
			continue
		}

		logger.V(4).Info("generate policy with clone, remove policy name from label of source resource")
		generatePolicyWithClone = true

		retryCount := 0
		for retryCount < 5 {
			err := updateSourceResource(pName, rule, client, logger)
			if err != nil {
				logger.Error(err, "failed to update generate source resource labels")
				if apierrors.IsConflict(err) {
					retryCount++
				} else {
					break
				}
			}
		}
	}

	return generatePolicyWithClone
}

func updateSourceResource(pName string, rule kyverno.Rule, client *dclient.Client, log logr.Logger) error {
	obj, err := client.GetResource("", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
	if err != nil {
		return errors.Wrapf(err, "source resource %s/%s/%s not found", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
	}

	update := false
	labels := obj.GetLabels()
	update, labels = removePolicyFromLabels(pName, labels)
	if update {
		return nil
	}

	obj.SetLabels(labels)
	_, err = client.UpdateResource(obj.GetAPIVersion(), rule.Generation.Kind, rule.Generation.Clone.Namespace, obj, false)
	return err
}

func removePolicyFromLabels(pName string, labels map[string]string) (bool, map[string]string) {
	if len(labels) == 0 {
		return false, labels
	}

	if labels["generate.kyverno.io/clone-policy-name"] != "" {
		policyNames := labels["generate.kyverno.io/clone-policy-name"]
		if strings.Contains(policyNames, pName) {
			updatedPolicyNames := strings.Replace(policyNames, pName, "", -1)
			labels["generate.kyverno.io/clone-policy-name"] = updatedPolicyNames
			return true, labels
		}
	}

	return false, labels
}

func GetFormatedKind(str string) (kind string) {
	if strings.Count(str, "/") == 0 {
		return strings.Title(str)
	}
	splitString := strings.Split(str, "/")
	if strings.Count(str, "/") == 1 {
		return splitString[0] + "/" + strings.Title(splitString[1])
	}
	return splitString[0] + "/" + splitString[1] + "/" + strings.Title(splitString[2])
}
