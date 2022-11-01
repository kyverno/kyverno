package common

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	enginutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// Policy Reporting Types
const (
	PolicyViolation = "POLICYVIOLATION"
	PolicyReport    = "POLICYREPORT"
)

// GetNamespaceSelectorsFromNamespaceLister - extract the namespacelabels when namespace lister is passed
func GetNamespaceSelectorsFromNamespaceLister(kind, namespaceOfResource string, nsLister corev1listers.NamespaceLister, logger logr.Logger) map[string]string {
	namespaceLabels := make(map[string]string)
	if kind != "Namespace" && namespaceOfResource != "" {
		namespaceObj, err := nsLister.Get(namespaceOfResource)
		if err != nil {
			logging.Error(err, "failed to get the namespace", "name", namespaceOfResource)
			return namespaceLabels
		}
		return GetNamespaceLabels(namespaceObj, logger)
	}
	return namespaceLabels
}

// GetNamespaceLabels - from namespace obj
func GetNamespaceLabels(namespaceObj *corev1.Namespace, logger logr.Logger) map[string]string {
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

func ProcessDeletePolicyForCloneGenerateRule(policy kyvernov1.PolicyInterface, client dclient.Interface, kyvernoClient versioned.Interface, urlister kyvernov1beta1listers.UpdateRequestNamespaceLister, pName string, logger logr.Logger) bool {
	generatePolicyWithClone := false
	for _, rule := range policy.GetSpec().Rules {
		clone, sync := rule.GetCloneSyncForGenerate()
		if !(clone && sync) {
			continue
		}

		logger.V(4).Info("generate policy with clone, remove policy name from label of source resource")
		generatePolicyWithClone = true

		var retryCount int
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

func updateSourceResource(pName string, rule kyvernov1.Rule, client dclient.Interface, log logr.Logger) error {
	obj, err := client.GetResource("", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
	if err != nil {
		return errors.Wrapf(err, "source resource %s/%s/%s not found", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
	}

	var update bool
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
