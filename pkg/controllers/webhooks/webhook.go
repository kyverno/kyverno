package webhooks

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/utils"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/pkg/errors"
	admregapi "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	apiGroups      string = "apiGroups"
	apiVersions    string = "apiVersions"
	resources      string = "resources"
	kindMutating   string = "MutatingWebhookConfiguration"
	kindValidating string = "ValidatingWebhookConfiguration"
)

// webhook is the instance that aggregates the GVK of existing policies
// based on kind, failurePolicy and webhookTimeout
type webhook struct {
	kind              string
	maxWebhookTimeout int64
	failurePolicy     kyverno.FailurePolicyType

	// rule represents the same rule struct of the webhook using a map object
	// https://github.com/kubernetes/api/blob/master/admissionregistration/v1/types.go#L25
	rule map[string]interface{}
}

func newWebhook(kind string, timeout int64, failurePolicy kyverno.FailurePolicyType) *webhook {
	return &webhook{
		kind:              kind,
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		rule:              make(map[string]interface{}),
	}
}

// mergeWebhook merges the matching kinds of the policy to webhook.rule
func mergeWebhook(dst *webhook, policy kyverno.PolicyInterface, d client.IDiscovery, updateValidate bool) {
	matchedGVK := make([]string, 0)
	for _, rule := range autogen.ComputeRules(policy) {
		// matching kinds in generate policies need to be added to both webhook
		if rule.HasGenerate() {
			matchedGVK = append(matchedGVK, rule.MatchResources.GetKinds()...)
			matchedGVK = append(matchedGVK, rule.Generation.ResourceSpec.Kind)
			continue
		}
		if (updateValidate && rule.HasValidate() || rule.HasImagesValidationChecks()) ||
			(updateValidate && rule.HasMutate() && rule.IsMutateExisting()) ||
			(!updateValidate && rule.HasMutate()) && !rule.IsMutateExisting() ||
			(!updateValidate && rule.HasVerifyImages()) {
			matchedGVK = append(matchedGVK, rule.MatchResources.GetKinds()...)
		}
	}
	gvkMap := make(map[string]int)
	gvrList := make([]schema.GroupVersionResource, 0)
	for _, gvk := range matchedGVK {
		if _, ok := gvkMap[gvk]; !ok {
			gvkMap[gvk] = 1
			// note: webhook stores GVR in its rules while policy stores GVK in its rules definition
			gv, k := kubeutils.GetKindFromGVK(gvk)
			switch k {
			case "Binding":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/binding"})
			case "NodeProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes/proxy"})
			case "PodAttachOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/attach"})
			case "PodExecOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/exec"})
			case "PodPortForwardOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/portforward"})
			case "PodProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/proxy"})
			case "ServiceProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services/proxy"})
			default:
				_, gvr, err := d.FindResource(gv, k)
				if err != nil {
					logger.Error(err, "unable to convert GVK to GVR", "GVK", gvk)
					continue
				}
				if strings.Contains(gvk, "*") {
					gvrList = append(gvrList, schema.GroupVersionResource{Group: gvr.Group, Version: "*", Resource: gvr.Resource})
				} else {
					gvrList = append(gvrList, gvr)
				}
			}
		}
	}
	var groups, versions, rsrcs []string
	if val, ok := dst.rule[apiGroups]; ok {
		groups = make([]string, len(val.([]string)))
		copy(groups, val.([]string))
	}
	if val, ok := dst.rule[apiVersions]; ok {
		versions = make([]string, len(val.([]string)))
		copy(versions, val.([]string))
	}
	if val, ok := dst.rule[resources]; ok {
		rsrcs = make([]string, len(val.([]string)))
		copy(rsrcs, val.([]string))
	}
	for _, gvr := range gvrList {
		groups = append(groups, gvr.Group)
		versions = append(versions, gvr.Version)
		rsrcs = append(rsrcs, gvr.Resource)
	}
	if utils.ContainsString(rsrcs, "pods") {
		rsrcs = append(rsrcs, "pods/ephemeralcontainers")
	}
	if len(groups) > 0 {
		dst.rule[apiGroups] = removeDuplicates(groups)
	}
	if len(versions) > 0 {
		dst.rule[apiVersions] = removeDuplicates(versions)
	}
	if len(rsrcs) > 0 {
		dst.rule[resources] = removeDuplicates(rsrcs)
	}
	spec := policy.GetSpec()
	if spec.WebhookTimeoutSeconds != nil {
		if dst.maxWebhookTimeout < int64(*spec.WebhookTimeoutSeconds) {
			dst.maxWebhookTimeout = int64(*spec.WebhookTimeoutSeconds)
		}
	}
}

func removeDuplicates(items []string) (res []string) {
	set := make(map[string]int)
	for _, item := range items {
		if _, ok := set[item]; !ok {
			set[item] = 1
			res = append(res, item)
		}
	}
	return
}

// webhookRulesEqual compares webhook rules between
// the representation returned by the API server,
// and the internal representation that is generated.
//
// The two representations are slightly different,
// so this function handles those differences.
func webhookRulesEqual(apiRules []interface{}, internalRules []interface{}) (bool, error) {
	// Handle edge case when both are empty.
	// API representation is a nil slice,
	// internal representation is one rule
	// but with no selectors.
	if len(apiRules) == 0 && len(internalRules) == 1 {
		if len(internalRules[0].(map[string]interface{})) == 0 {
			return true, nil
		}
	}

	// Handle edge case when internal is empty but API has one rule.
	// internal representation is one rule but with no selectors.
	if len(apiRules) == 1 && len(internalRules) == 1 {
		if len(internalRules[0].(map[string]interface{})) == 0 {
			return false, nil
		}
	}

	// Both *should* be length 1, but as long
	// as they are equal the next loop works.
	if len(apiRules) != len(internalRules) {
		return false, nil
	}

	for i := range internalRules {
		internal, ok := internalRules[i].(map[string]interface{})
		if !ok {
			return false, errors.New("type conversion of internal rules failed")
		}
		api, ok := apiRules[i].(map[string]interface{})
		if !ok {
			return false, errors.New("type conversion of API rules failed")
		}

		// Range over the fields of internal, as the
		// API rule has extra fields (operations, scope)
		// that can't be checked on the internal rules.
		for field := range internal {
			// Convert the API rules values to []string.
			apiValues, _, err := unstructured.NestedStringSlice(api, field)
			if err != nil {
				return false, errors.Wrapf(err, "error getting string slice for API rules field %s", field)
			}

			// Internal type is already []string.
			internalValues := internal[field]

			if !reflect.DeepEqual(internalValues, apiValues) {
				return false, nil
			}
		}
	}

	return true, nil
}

func (m *controller) getWebhook(webhookKind, webhookName string) (resourceWebhook *unstructured.Unstructured, err error) {
	get := func() error {
		resourceWebhook = &unstructured.Unstructured{}
		err = nil

		var rawResc []byte

		switch webhookKind {
		case kindMutating:
			resourceWebhookTyped, err := m.mutateLister.Get(webhookName)
			if err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "unable to get %s/%s", webhookKind, webhookName)
			} else if apierrors.IsNotFound(err) {
				m.createDefaultWebhook <- webhookKind
				return err
			}
			resourceWebhookTyped.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "admissionregistration.k8s.io/v1", Kind: kindMutating})
			rawResc, err = json.Marshal(resourceWebhookTyped)
			if err != nil {
				return err
			}
		case kindValidating:
			resourceWebhookTyped, err := m.validateLister.Get(webhookName)
			if err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "unable to get %s/%s", webhookKind, webhookName)
			} else if apierrors.IsNotFound(err) {
				m.createDefaultWebhook <- webhookKind
				return err
			}
			resourceWebhookTyped.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "admissionregistration.k8s.io/v1", Kind: kindValidating})
			rawResc, err = json.Marshal(resourceWebhookTyped)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown webhook kind: must be '%v' or '%v'", kindMutating, kindValidating)
		}

		err = json.Unmarshal(rawResc, &resourceWebhook.Object)

		return err
	}

	msg := "getWebhook: unable to get webhook configuration"
	retryGetWebhook := common.RetryFunc(time.Second, 10*time.Second, get, msg, logger)
	if err := retryGetWebhook(); err != nil {
		return nil, err
	}

	return resourceWebhook, nil
}

func (m *controller) compareAndUpdateWebhook(webhookKind, webhookName string, webhooksMap map[string]interface{}) error {
	logger := logger.WithName("compareAndUpdateWebhook").WithValues("kind", webhookKind, "name", webhookName)
	resourceWebhook, err := m.getWebhook(webhookKind, webhookName)
	if err != nil {
		return err
	}

	webhooksUntyped, _, err := unstructured.NestedSlice(resourceWebhook.UnstructuredContent(), "webhooks")
	if err != nil {
		return errors.Wrapf(err, "unable to fetch tag webhooks for %s/%s", webhookKind, webhookName)
	}

	newWebooks := make([]interface{}, len(webhooksUntyped))
	copy(newWebooks, webhooksUntyped)
	var changed bool
	for i, webhookUntyed := range webhooksUntyped {
		existingWebhook, ok := webhookUntyed.(map[string]interface{})
		if !ok {
			logger.Error(errors.New("type mismatched"), "expected map[string]interface{}, got %T", webhooksUntyped)
			continue
		}

		failurePolicy, _, err := unstructured.NestedString(existingWebhook, "failurePolicy")
		if err != nil {
			logger.Error(errors.New("type mismatched"), "expected string, got %T", failurePolicy)
			continue

		}

		rules, _, err := unstructured.NestedSlice(existingWebhook, "rules")
		if err != nil {
			logger.Error(err, "type mismatched, expected []interface{}, got %T", rules)
			continue
		}

		newWebhook := webhooksMap[webhookKey(webhookKind, failurePolicy)]
		w, ok := newWebhook.(*webhook)
		if !ok {
			logger.Error(errors.New("type mismatched"), "expected *webhook, got %T", newWebooks)
			continue
		}

		rulesEqual, err := webhookRulesEqual(rules, []interface{}{w.rule})
		if err != nil {
			logger.Error(err, "failed to compare webhook rules")
			continue
		}

		if !rulesEqual {
			changed = true

			tmpRules, ok := newWebooks[i].(map[string]interface{})["rules"].([]interface{})
			if !ok {
				// init operations
				ops := []string{string(admregapi.Create), string(admregapi.Update), string(admregapi.Delete), string(admregapi.Connect)}
				if webhookKind == kindMutating {
					ops = []string{string(admregapi.Create), string(admregapi.Update), string(admregapi.Delete)}
				}

				tmpRules = []interface{}{map[string]interface{}{}}
				if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), ops, "operations"); err != nil {
					return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, apiGroups)
				}
			}

			if w.rule == nil || reflect.DeepEqual(w.rule, map[string]interface{}{}) {
				// zero kyverno policy with the current failurePolicy, reset webhook rules to empty
				newWebooks[i].(map[string]interface{})["rules"] = []interface{}{}
				continue
			}

			if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), w.rule[apiGroups].([]string), apiGroups); err != nil {
				return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, apiGroups)
			}
			if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), w.rule[apiVersions].([]string), apiVersions); err != nil {
				return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, apiVersions)
			}
			if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), w.rule[resources].([]string), resources); err != nil {
				return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, resources)
			}

			newWebooks[i].(map[string]interface{})["rules"] = tmpRules
		}

		if err = unstructured.SetNestedField(newWebooks[i].(map[string]interface{}), w.maxWebhookTimeout, "timeoutSeconds"); err != nil {
			return errors.Wrapf(err, "unable to set webhooks[%d].timeoutSeconds to %v", i, w.maxWebhookTimeout)
		}
	}

	if changed {
		logger.V(4).Info("webhook configuration has been changed, updating")
		if err := unstructured.SetNestedSlice(resourceWebhook.UnstructuredContent(), newWebooks, "webhooks"); err != nil {
			return errors.Wrap(err, "unable to set new webhooks")
		}

		if _, err := m.client.UpdateResource(resourceWebhook.GetAPIVersion(), resourceWebhook.GetKind(), "", resourceWebhook, false); err != nil {
			return errors.Wrapf(err, "unable to update %s/%s: %s", resourceWebhook.GetAPIVersion(), resourceWebhook.GetKind(), resourceWebhook.GetName())
		}

		logger.V(4).Info("successfully updated the webhook configuration")
	}

	return nil
}

// func (m *webhookConfigManager) updateStatus(namespace, name string, ready bool) error {
// 	update := func(meta *metav1.ObjectMeta, spec *kyverno.Spec, status *kyverno.PolicyStatus) bool {
// 		copy := status.DeepCopy()
// 		requested, _, activated := autogen.GetControllers(meta, spec)
// 		status.SetReady(ready)
// 		status.Autogen.Requested = requested
// 		status.Autogen.Activated = activated
// 		status.Rules = spec.Rules
// 		return !reflect.DeepEqual(status, copy)
// 	}
// 	if namespace == "" {
// 		p, err := m.pLister.Get(name)
// 		if err != nil {
// 			return err
// 		}
// 		if update(&p.ObjectMeta, &p.Spec, &p.Status) {
// 			if _, err := m.kyvernoClient.KyvernoV1().ClusterPolicies().UpdateStatus(context.TODO(), p, metav1.UpdateOptions{}); err != nil {
// 				return err
// 			}
// 		}
// 	} else {
// 		p, err := m.npLister.Policies(namespace).Get(name)
// 		if err != nil {
// 			return err
// 		}
// 		if update(&p.ObjectMeta, &p.Spec, &p.Status) {
// 			if _, err := m.kyvernoClient.KyvernoV1().Policies(namespace).UpdateStatus(context.TODO(), p, metav1.UpdateOptions{}); err != nil {
// 				return err
// 			}
// 		}
// 	}
// 	return nil
// }

func webhookKey(webhookKind, failurePolicy string) string {
	return strings.Join([]string{webhookKind, failurePolicy}, "/")
}

func setWildcardConfig(w *webhook) {
	w.rule[apiGroups] = []string{"*"}
	w.rule[apiVersions] = []string{"*"}
	w.rule[resources] = []string{"*/*"}
}

func getResourceMutatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.MutatingWebhookConfigurationDebugName
	}
	return config.MutatingWebhookConfigurationName
}

func getResourceValidatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.ValidatingWebhookConfigurationDebugName
	}

	return config.ValidatingWebhookConfigurationName
}
