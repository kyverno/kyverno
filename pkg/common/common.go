package common

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	urkyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	enginutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/informers"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	if kind != "Namespace" && namespaceOfResource != "" {
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

// RetryFunc allows retrying a function on error within a given timeout
func RetryFunc(retryInterval, timeout time.Duration, run func() error, msg string, logger logr.Logger) func() error {
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
					logger.V(3).Info(msg, "reason", err.Error())
				} else {
					break loop
				}

			case <-registerTimeout:
				return errors.Wrap(err, "retry times out")
			}
		}
		return nil
	}
}

func ProcessDeletePolicyForCloneGenerateRule(policy kyverno.PolicyInterface, client dclient.Interface, kyvernoClient kyvernoclient.Interface, urlister urkyvernolister.UpdateRequestNamespaceLister, pName string, logger logr.Logger) bool {
	generatePolicyWithClone := false
	for _, rule := range policy.GetSpec().Rules {
		clone, sync := rule.GetCloneSyncForGenerate()
		if !(clone && sync) {
			continue
		}

		logger.V(4).Info("generate policy with clone, remove policy name from label of source resource")
		generatePolicyWithClone = true

		retryCount := 0
		for retryCount < 5 {
			err := updateSourceResource(policy.GetName(), rule, client, logger)
			if err != nil {
				logger.Error(err, "failed to update generate source resource labels")
				if apierrors.IsConflict(err) {
					retryCount++
				} else {
					break
				}
			}
			break
		}
	}

	return generatePolicyWithClone
}

func updateSourceResource(pName string, rule kyverno.Rule, client dclient.Interface, log logr.Logger) error {
	obj, err := client.GetResource("", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
	if err != nil {
		return errors.Wrapf(err, "source resource %s/%s/%s not found", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
	}

	update := false
	labels := obj.GetLabels()
	update, labels = removePolicyFromLabels(pName, labels)
	if !update {
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
			desiredLabels := make(map[string]string, len(labels)-1)
			for k, v := range labels {
				if k != "generate.kyverno.io/clone-policy-name" {
					desiredLabels[k] = v
				}
			}

			return true, desiredLabels
		}
	}

	return false, labels
}
