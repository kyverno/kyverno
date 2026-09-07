package admissionpolicygenerator

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// vapVariant is one desired ValidatingAdmissionPolicy for a policy: either the base
// (unmodified) policy, or the rewritten rule for one autogen group (e.g. "defaults",
// "cronjobs"). group/specOverride are "" / nil for the base variant. group is also needed
// (independent of specOverride) to correctly rewrite any PolicyException embedded into this
// variant - see BuildValidatingAdmissionPolicy's group parameter.
type vapVariant struct {
	name         string
	group        string
	specOverride *policiesv1beta1.ValidatingPolicySpec
}

// desiredVAPVariants returns the ValidatingAdmissionPolicies that should exist for a
// ValidatingPolicy: the base policy plus one per translatable autogen group. Extraction-mode
// (custom-CRD) groups are excluded and returned separately as skippedGroups - they have no
// fixed field path a native VAP's CEL could address, since their pod template is only ever
// located by Kyverno's own Go code at evaluation time (see pkg/cel/autogen/extract), which a
// native VAP has no equivalent of.
func desiredVAPVariants(vpol *policiesv1beta1.ValidatingPolicy, baseName string) (variants []vapVariant, skippedGroups []string) {
	variants = append(variants, vapVariant{name: baseName})
	configs := vpol.GetStatus().Autogen.Configs
	for _, group := range slices.Sorted(maps.Keys(configs)) {
		if group == vpolautogen.ExtractionReplacementsRef {
			skippedGroups = append(skippedGroups, group)
			continue
		}
		variants = append(variants, vapVariant{name: baseName + "-" + group, group: group, specOverride: configs[group].Spec})
	}
	return variants, skippedGroups
}

func (c *controller) handleVAPGeneration(ctx context.Context, polType string, policy engineapi.GenericPolicy) error {
	// check if the controller has the required permissions to generate ValidatingAdmissionPolicies.
	if !admissionpolicy.HasValidatingAdmissionPolicyPermission(c.checker) {
		logger.V(2).Info("insufficient permissions to generate ValidatingAdmissionPolicies")
		c.updatePolicyStatus(ctx, policy, false, "insufficient permissions to generate ValidatingAdmissionPolicies")
		return nil
	}
	// check if the controller has the required permissions to generate ValidatingAdmissionPolicyBindings.
	if !admissionpolicy.HasValidatingAdmissionPolicyBindingPermission(c.checker) {
		logger.V(2).Info("insufficient permissions to generate ValidatingAdmissionPolicyBindings")
		c.updatePolicyStatus(ctx, policy, false, "insufficient permissions to generate ValidatingAdmissionPolicyBindings")
		return nil
	}

	if polType == "ClusterPolicy" {
		return c.handleVAPGenerationForClusterPolicy(ctx, policy)
	}
	return c.handleVAPGenerationForValidatingPolicy(ctx, policy)
}

// handleVAPGenerationForClusterPolicy generates a single ValidatingAdmissionPolicy from a legacy
// ClusterPolicy. Unchanged from before the autogen fan-out work - ClusterPolicy's own (legacy,
// annotation-based) autogen mechanism is a structurally different problem, out of scope here.
func (c *controller) handleVAPGenerationForClusterPolicy(ctx context.Context, policy engineapi.GenericPolicy) error {
	vapName := "cpol-" + policy.GetName()
	vapBindingName := constructBindingName(vapName)
	observedVAP, vapErr := c.getValidatingAdmissionPolicy(vapName)
	observedVAPbinding, vapBindingErr := c.getValidatingAdmissionPolicyBinding(vapBindingName)

	spec := policy.AsKyvernoPolicy().GetSpec()
	exceptions, err := c.getExceptions(policy.GetName(), spec.Rules[0].Name)
	if err != nil {
		return err
	}

	if ok, msg := admissionpolicy.CanGenerateVAP(spec, exceptions, false); !ok {
		if vapErr == nil {
			if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, vapName, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
		if vapBindingErr == nil {
			if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, vapBindingName, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
		if msg == "" {
			msg = "skip generating ValidatingAdmissionPolicy: a policy exception is configured."
		}
		c.updatePolicyStatus(ctx, policy, false, msg)
		return nil
	}

	genericExceptions := make([]engineapi.GenericException, 0, len(exceptions))
	for _, exception := range exceptions {
		genericExceptions = append(genericExceptions, engineapi.NewPolicyException(&exception))
	}

	if err := c.applyVAP(ctx, vapVariant{name: vapName}, vapBindingName, observedVAP, vapErr, observedVAPbinding, vapBindingErr, policy, genericExceptions); err != nil {
		return err
	}

	c.updatePolicyStatus(ctx, policy, true, "")
	c.eventGen.Add(event.NewValidatingAdmissionPolicyEvent(policy, vapName, vapBindingName)...)
	return nil
}

// handleVAPGenerationForValidatingPolicy generates one ValidatingAdmissionPolicy per autogen
// group (the "fan-out" fix for #17423) instead of the old all-or-nothing behavior that skipped
// VAP generation entirely whenever pod-controller autogen was configured.
func (c *controller) handleVAPGenerationForValidatingPolicy(ctx context.Context, policy engineapi.GenericPolicy) error {
	pol := policy.AsValidatingPolicy()
	vapName := "vpol-" + policy.GetName()

	if !pol.GetSpec().GenerateValidatingAdmissionPolicyEnabled() {
		if err := c.deleteVAPVariants(ctx, nil, policy, vapName); err != nil {
			return err
		}
		c.updatePolicyStatus(ctx, policy, false, "skip generating ValidatingAdmissionPolicy: not enabled.")
		return nil
	}

	celexceptions, err := c.getCELExceptions(policy.GetName())
	if err != nil {
		return fmt.Errorf("failed to get celexceptions by name %s: %v", policy.GetName(), err)
	}
	genericExceptions := make([]engineapi.GenericException, 0, len(celexceptions))
	for _, exception := range celexceptions {
		genericExceptions = append(genericExceptions, engineapi.NewCELPolicyException(&exception))
	}

	variants, skippedGroups := desiredVAPVariants(pol, vapName)

	var generateErrs []string
	for _, variant := range variants {
		vapBindingName := constructBindingName(variant.name)
		observedVAP, vapErr := c.getValidatingAdmissionPolicy(variant.name)
		observedVAPbinding, vapBindingErr := c.getValidatingAdmissionPolicyBinding(vapBindingName)
		if err := c.applyVAP(ctx, variant, vapBindingName, observedVAP, vapErr, observedVAPbinding, vapBindingErr, policy, genericExceptions); err != nil {
			generateErrs = append(generateErrs, fmt.Sprintf("%s: %v", variant.name, err))
			continue
		}
		c.eventGen.Add(event.NewValidatingAdmissionPolicyEvent(policy, variant.name, vapBindingName)...)
	}

	if err := c.deleteVAPVariants(ctx, variants, policy, vapName); err != nil {
		generateErrs = append(generateErrs, err.Error())
	}

	if len(generateErrs) > 0 {
		err := fmt.Errorf("failed to generate ValidatingAdmissionPolicy: %s", strings.Join(generateErrs, "; "))
		c.updatePolicyStatus(ctx, policy, false, err.Error())
		return err
	}

	// A generated VAP only replaces Kyverno's own webhook/background-scan evaluation when every
	// autogen target was actually translated - if a custom-CRD (extraction-mode) group was
	// skipped, that group is only ever enforced by Kyverno's webhook (see pkg/cel/autogen/extract),
	// so status.generated must stay false to keep Kyverno's own engine evaluating this policy too.
	// This does mean the built-in-controller groups end up double-covered (both a generated VAP
	// and Kyverno's webhook) whenever a custom CRD is also autogen'd on the same policy - a real,
	// accepted inefficiency traded for correctness in that mixed case.
	if len(skippedGroups) > 0 {
		msg := fmt.Sprintf("ValidatingAdmissionPolicy generated for the pod-controller autogen target(s); custom-CRD autogen target(s) (%s) cannot be translated to a native ValidatingAdmissionPolicy and remain enforced via Kyverno's webhook only.", strings.Join(skippedGroups, ", "))
		c.updatePolicyStatus(ctx, policy, false, msg)
		return nil
	}

	c.updatePolicyStatus(ctx, policy, true, "")
	return nil
}

// applyVAP creates or updates a single ValidatingAdmissionPolicy + binding for one variant
// (the base policy or one autogen group).
func (c *controller) applyVAP(
	ctx context.Context,
	variant vapVariant,
	vapBindingName string,
	observedVAP *admissionregistrationv1.ValidatingAdmissionPolicy,
	vapErr error,
	observedVAPbinding *admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	vapBindingErr error,
	policy engineapi.GenericPolicy,
	genericExceptions []engineapi.GenericException,
) error {
	if vapErr != nil {
		if !apierrors.IsNotFound(vapErr) {
			return fmt.Errorf("failed to get validatingadmissionpolicy %s: %v", variant.name, vapErr)
		}
		observedVAP = &admissionregistrationv1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: variant.name},
		}
	}
	if vapBindingErr != nil {
		if !apierrors.IsNotFound(vapBindingErr) {
			return fmt.Errorf("failed to get validatingadmissionpolicybinding %s: %v", vapBindingName, vapBindingErr)
		}
		observedVAPbinding = &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: vapBindingName},
		}
	}

	if observedVAP.ResourceVersion == "" {
		if err := admissionpolicy.BuildValidatingAdmissionPolicy(c.discoveryClient, observedVAP, policy, genericExceptions, variant.group, variant.specOverride); err != nil {
			return fmt.Errorf("failed to build validatingadmissionpolicy %s: %v", variant.name, err)
		}
		if _, err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, observedVAP, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create validatingadmissionpolicy %s: %v", variant.name, err)
		}
	} else {
		if _, err := controllerutils.Update(
			ctx,
			observedVAP,
			c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies(),
			func(observed *admissionregistrationv1.ValidatingAdmissionPolicy) error {
				return admissionpolicy.BuildValidatingAdmissionPolicy(c.discoveryClient, observed, policy, genericExceptions, variant.group, variant.specOverride)
			}); err != nil {
			return fmt.Errorf("failed to update validatingadmissionpolicy %s: %v", variant.name, err)
		}
	}

	if observedVAPbinding.ResourceVersion == "" {
		if err := admissionpolicy.BuildValidatingAdmissionPolicyBinding(observedVAPbinding, policy, variant.name, variant.specOverride); err != nil {
			return fmt.Errorf("failed to build validatingadmissionpolicybinding %s: %v", vapBindingName, err)
		}
		if _, err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, observedVAPbinding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create validatingadmissionpolicybinding %s: %v", vapBindingName, err)
		}
	} else {
		if _, err := controllerutils.Update(
			ctx,
			observedVAPbinding,
			c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings(),
			func(observed *admissionregistrationv1.ValidatingAdmissionPolicyBinding) error {
				return admissionpolicy.BuildValidatingAdmissionPolicyBinding(observed, policy, variant.name, variant.specOverride)
			}); err != nil {
			return fmt.Errorf("failed to update validatingadmissionpolicybinding %s: %v", vapBindingName, err)
		}
	}
	return nil
}

// deleteVAPVariants removes any ValidatingAdmissionPolicy (+ binding) previously generated for
// this policy that isn't in the current desired set - e.g. an autogen group was removed from
// spec.autogen.podControllers.controllers, or VAP generation was disabled entirely (desired ==
// nil). Ownership is checked via OwnerReferences UID, not just the baseName prefix, so this can
// never touch another policy's generated objects even if names happen to share a prefix (e.g.
// policy "foo" vs. policy "foo-defaults").
func (c *controller) deleteVAPVariants(ctx context.Context, desired []vapVariant, policy engineapi.GenericPolicy, baseName string) error {
	desiredNames := make(map[string]bool, len(desired))
	for _, v := range desired {
		desiredNames[v.name] = true
	}

	owned, err := c.listOwnedVAPNames(policy, baseName)
	if err != nil {
		return err
	}
	for _, name := range owned {
		if desiredNames[name] {
			continue
		}
		bindingName := constructBindingName(name)
		if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale validatingadmissionpolicy %s: %v", name, err)
		}
		if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, bindingName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale validatingadmissionpolicybinding %s: %v", bindingName, err)
		}
	}
	return nil
}

// listOwnedVAPNames returns the names of every ValidatingAdmissionPolicy owned by policy (by
// OwnerReferences UID) whose name is either baseName ("vpol-<name>"/"cpol-<name>", the base
// variant) or "<baseName>-<group>" (an autogen-group variant). The name check is just a cheap
// pre-filter; UID is what actually decides ownership.
func (c *controller) listOwnedVAPNames(policy engineapi.GenericPolicy, baseName string) ([]string, error) {
	if c.vapLister == nil {
		return nil, nil
	}
	all, err := c.vapLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list validatingadmissionpolicies: %v", err)
	}
	uid := policy.GetUID()
	prefix := baseName + "-"
	var names []string
	for _, vap := range all {
		if vap.Name != baseName && !strings.HasPrefix(vap.Name, prefix) {
			continue
		}
		for _, ref := range vap.OwnerReferences {
			if ref.UID == uid {
				names = append(names, vap.Name)
				break
			}
		}
	}
	return names, nil
}
