package admissionpolicygenerator

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	mpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/mpol/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// mapVariant is one desired MutatingAdmissionPolicy for a policy: either the base (unmodified)
// policy, or the rewritten rule for one autogen group (e.g. "defaults", "cronjobs").
// group/specOverride are "" / nil for the base variant. group is also needed (independent of
// specOverride) to correctly rewrite any PolicyException embedded into this variant - see
// BuildMutatingAdmissionPolicy's group parameter.
type mapVariant struct {
	name         string
	group        string
	specOverride *policiesv1beta1.MutatingPolicySpec
}

// desiredMAPVariants mirrors desiredVAPVariants (generate-vap.go) for MutatingPolicy: the base
// policy plus one per translatable autogen group, with extraction-mode (custom-CRD) groups
// excluded and reported back separately - see desiredVAPVariants' doc comment for why.
func desiredMAPVariants(mpol *policiesv1beta1.MutatingPolicy, baseName string) (variants []mapVariant, skippedGroups []string) {
	variants = append(variants, mapVariant{name: baseName})
	configs := mpol.GetStatus().Autogen.Configs
	for _, group := range slices.Sorted(maps.Keys(configs)) {
		if group == mpolautogen.ExtractionReplacementsRef {
			skippedGroups = append(skippedGroups, group)
			continue
		}
		variants = append(variants, mapVariant{name: baseName + "-" + group, group: group, specOverride: configs[group].Spec})
	}
	return variants, skippedGroups
}

// preferredMAPVersion returns the API version to use based on which listers are initialised.
// Both the policy and binding listers must be set for a version to be considered available.
func (c *controller) preferredMAPVersion() (admissionpolicy.MutatingAdmissionPolicyVersion, bool) {
	if c.mapV1Lister != nil && c.mapbindingV1Lister != nil {
		return admissionpolicy.MutatingAdmissionPolicyVersionV1, true
	}
	if c.mapBetaLister != nil && c.mapbindingBetaLister != nil {
		return admissionpolicy.MutatingAdmissionPolicyVersionV1beta1, true
	}
	if c.mapAlphaLister != nil && c.mapbindingAlphaLister != nil {
		return admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1, true
	}
	return "", false
}

func (c *controller) handleMAPGeneration(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy) error {
	version, ok := c.preferredMAPVersion()
	if !ok {
		logger.V(2).Info("No MutatingAdmissionPolicy lister available, skipping MAP generation", "policy", mpol.GetName())
		if mpol.GetSpec().GenerateMutatingAdmissionPolicyEnabled() {
			genericPolicy := engineapi.NewMutatingPolicy(mpol)
			c.updatePolicyStatus(ctx, genericPolicy, false, "skip generating MutatingAdmissionPolicy: requested but no MutatingAdmissionPolicy API/informers are available.")
		}
		return nil
	}
	logger.V(4).Info("Using MutatingAdmissionPolicy API", "version", version, "policy", mpol.GetName())
	return c.handleMAPGenerationWithVersion(ctx, mpol, version)
}

// mapFullSkipReason returns a non-empty reason when a MutatingAdmissionPolicy must not be
// generated at all for the policy (any previously generated variant is deleted). Unlike pod
// controller autogen (handled per-group by the fan-out below), these are whole-policy blockers:
// useServerSideApply mutates atomic fields that a native MutatingAdmissionPolicy rejects, so a
// generated MAP (which becomes the sole admission path once status.generated is set) would drop
// the mutation.
func mapFullSkipReason(mpol *policiesv1beta1.MutatingPolicy) string {
	if !mpol.GetSpec().GenerateMutatingAdmissionPolicyEnabled() {
		return "skip generating MutatingAdmissionPolicy: not enabled."
	}
	if ec := mpol.GetSpec().EvaluationConfiguration; ec != nil && ec.UseServerSideApply {
		return "skip generating MutatingAdmissionPolicy: useServerSideApply is enabled, which mutates atomic fields that a native MutatingAdmissionPolicy rejects."
	}
	return ""
}

func (c *controller) handleMAPGenerationWithVersion(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, version admissionpolicy.MutatingAdmissionPolicyVersion) error {
	genericPolicy := engineapi.NewMutatingPolicy(mpol)
	if !admissionpolicy.HasMutatingAdmissionPolicyPermissionForVersion(version, c.checker) {
		logger.V(2).Info("insufficient permissions to generate MutatingAdmissionPolicies")
		c.updatePolicyStatus(ctx, genericPolicy, false, "insufficient permissions to generate MutatingAdmissionPolicies")
		return nil
	}
	if !admissionpolicy.HasMutatingAdmissionPolicyBindingPermissionForVersion(version, c.checker) {
		logger.V(2).Info("insufficient permissions to generate MutatingAdmissionPolicyBindings")
		c.updatePolicyStatus(ctx, genericPolicy, false, "insufficient permissions to generate MutatingAdmissionPolicyBindings")
		return nil
	}

	mapName := "mpol-" + mpol.GetName()

	if reason := mapFullSkipReason(mpol); reason != "" {
		if err := c.deleteMAPVariants(ctx, nil, genericPolicy, mapName, version); err != nil {
			return err
		}
		c.updatePolicyStatus(ctx, genericPolicy, false, reason)
		return nil
	}

	celexceptions, err := c.getCELExceptions(mpol.GetName())
	if err != nil {
		return fmt.Errorf("failed to get celexceptions by name %s: %v", mpol.GetName(), err)
	}

	variants, skippedGroups := desiredMAPVariants(mpol, mapName)

	var generateErrs []string
	for _, variant := range variants {
		if err := c.applyMAPVariant(ctx, version, variant, mpol, celexceptions); err != nil {
			generateErrs = append(generateErrs, fmt.Sprintf("%s: %v", variant.name, err))
			continue
		}
	}
	if err := c.deleteMAPVariants(ctx, variants, genericPolicy, mapName, version); err != nil {
		generateErrs = append(generateErrs, err.Error())
	}

	if len(generateErrs) > 0 {
		err := fmt.Errorf("failed to generate MutatingAdmissionPolicy: %s", strings.Join(generateErrs, "; "))
		c.updatePolicyStatus(ctx, genericPolicy, false, err.Error())
		return err
	}

	// See the identical comment in handleVAPGenerationForValidatingPolicy (generate-vap.go): a
	// custom-CRD autogen target can never become a native MAP, so status.generated must stay
	// false whenever one is present, or Kyverno's own webhook/background-scan would stop
	// evaluating this policy for that target entirely.
	if len(skippedGroups) > 0 {
		msg := fmt.Sprintf("MutatingAdmissionPolicy generated for the pod-controller autogen target(s); custom-CRD autogen target(s) (%s) cannot be translated to a native MutatingAdmissionPolicy and remain enforced via Kyverno's webhook only.", strings.Join(skippedGroups, ", "))
		c.updatePolicyStatus(ctx, genericPolicy, false, msg)
		return nil
	}

	c.updatePolicyStatus(ctx, genericPolicy, true, "")
	return nil
}

// applyMAPVariant creates or updates a single MutatingAdmissionPolicy + binding, for whichever
// API version is preferred in this cluster.
func (c *controller) applyMAPVariant(ctx context.Context, version admissionpolicy.MutatingAdmissionPolicyVersion, variant mapVariant, mpol *policiesv1beta1.MutatingPolicy, celexceptions []policiesv1beta1.PolicyException) error {
	switch version {
	case admissionpolicy.MutatingAdmissionPolicyVersionV1:
		return c.applyMAPV1(ctx, variant, mpol, celexceptions)
	case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
		return c.applyMAPBeta(ctx, variant, mpol, celexceptions)
	case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
		return c.applyMAPAlpha(ctx, variant, mpol, celexceptions)
	default:
		return fmt.Errorf("unsupported MutatingAdmissionPolicy version: %s", version)
	}
}

func (c *controller) applyMAPV1(ctx context.Context, variant mapVariant, mpol *policiesv1beta1.MutatingPolicy, celexceptions []policiesv1beta1.PolicyException) error {
	mapBindingName := constructBindingName(variant.name)
	observedMAP, mapErr := c.getMutatingAdmissionPolicyV1(variant.name)
	observedMAPbinding, mapBindingErr := c.getMutatingAdmissionPolicyBindingV1(mapBindingName)

	if mapErr != nil {
		if !apierrors.IsNotFound(mapErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicy %s: %v", variant.name, mapErr)
		}
		observedMAP = &admissionregistrationv1.MutatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: variant.name}}
	}
	if mapBindingErr != nil {
		if !apierrors.IsNotFound(mapBindingErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicybinding %s: %v", mapBindingName, mapBindingErr)
		}
		observedMAPbinding = &admissionregistrationv1.MutatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: mapBindingName}}
	}

	if observedMAP.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyV1(observedMAP, mpol, celexceptions, variant.group, variant.specOverride)
		if _, err := c.client.AdmissionregistrationV1().MutatingAdmissionPolicies().Create(ctx, observedMAP, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicy %s: %v", variant.name, err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAP, c.client.AdmissionregistrationV1().MutatingAdmissionPolicies(), func(observed *admissionregistrationv1.MutatingAdmissionPolicy) error {
			admissionpolicy.BuildMutatingAdmissionPolicyV1(observed, mpol, celexceptions, variant.group, variant.specOverride)
			return nil
		}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicy %s: %v", variant.name, err)
		}
	}

	if observedMAPbinding.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyBindingV1(observedMAPbinding, mpol, variant.name)
		if _, err := c.client.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Create(ctx, observedMAPbinding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicybinding %s: %v", mapBindingName, err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAPbinding, c.client.AdmissionregistrationV1().MutatingAdmissionPolicyBindings(), func(observed *admissionregistrationv1.MutatingAdmissionPolicyBinding) error {
			admissionpolicy.BuildMutatingAdmissionPolicyBindingV1(observed, mpol, variant.name)
			return nil
		}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicybinding %s: %v", mapBindingName, err)
		}
	}
	return nil
}

func (c *controller) applyMAPAlpha(ctx context.Context, variant mapVariant, mpol *policiesv1beta1.MutatingPolicy, celexceptions []policiesv1beta1.PolicyException) error {
	mapBindingName := constructBindingName(variant.name)
	observedMAP, mapErr := c.getMutatingAdmissionPolicy(variant.name)
	observedMAPbinding, mapBindingErr := c.getMutatingAdmissionPolicyBinding(mapBindingName)

	if mapErr != nil {
		if !apierrors.IsNotFound(mapErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicy %s: %v", variant.name, mapErr)
		}
		observedMAP = &admissionregistrationv1alpha1.MutatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: variant.name}}
	}
	if mapBindingErr != nil {
		if !apierrors.IsNotFound(mapBindingErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicybinding %s: %v", mapBindingName, mapBindingErr)
		}
		observedMAPbinding = &admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: mapBindingName}}
	}

	if observedMAP.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicy(observedMAP, mpol, celexceptions, variant.group, variant.specOverride)
		if _, err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Create(ctx, observedMAP, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicy %s: %v", variant.name, err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAP, c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies(),
			func(observed *admissionregistrationv1alpha1.MutatingAdmissionPolicy) error {
				admissionpolicy.BuildMutatingAdmissionPolicy(observed, mpol, celexceptions, variant.group, variant.specOverride)
				return nil
			}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicy %s: %v", variant.name, err)
		}
	}

	if observedMAPbinding.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyBinding(observedMAPbinding, mpol, variant.name)
		if _, err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings().Create(ctx, observedMAPbinding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicybinding %s: %v", mapBindingName, err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAPbinding, c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings(),
			func(observed *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) error {
				admissionpolicy.BuildMutatingAdmissionPolicyBinding(observed, mpol, variant.name)
				return nil
			}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicybinding %s: %v", mapBindingName, err)
		}
	}
	return nil
}

func (c *controller) applyMAPBeta(ctx context.Context, variant mapVariant, mpol *policiesv1beta1.MutatingPolicy, celexceptions []policiesv1beta1.PolicyException) error {
	mapBindingName := constructBindingName(variant.name)
	observedMAP, mapErr := c.getMutatingAdmissionPolicyBeta(variant.name)
	observedMAPbinding, mapBindingErr := c.getMutatingAdmissionPolicyBindingBeta(mapBindingName)

	if mapErr != nil {
		if !apierrors.IsNotFound(mapErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicy %s: %v", variant.name, mapErr)
		}
		observedMAP = &admissionregistrationv1beta1.MutatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: variant.name}}
	}
	if mapBindingErr != nil {
		if !apierrors.IsNotFound(mapBindingErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicybinding %s: %v", mapBindingName, mapBindingErr)
		}
		observedMAPbinding = &admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: mapBindingName}}
	}

	if observedMAP.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyBeta(observedMAP, mpol, celexceptions, variant.group, variant.specOverride)
		if _, err := c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Create(ctx, observedMAP, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicy %s: %v", variant.name, err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAP, c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicies(),
			func(observed *admissionregistrationv1beta1.MutatingAdmissionPolicy) error {
				admissionpolicy.BuildMutatingAdmissionPolicyBeta(observed, mpol, celexceptions, variant.group, variant.specOverride)
				return nil
			}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicy %s: %v", variant.name, err)
		}
	}

	if observedMAPbinding.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyBindingBeta(observedMAPbinding, mpol, variant.name)
		if _, err := c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings().Create(ctx, observedMAPbinding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicybinding %s: %v", mapBindingName, err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAPbinding, c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings(),
			func(observed *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding) error {
				admissionpolicy.BuildMutatingAdmissionPolicyBindingBeta(observed, mpol, variant.name)
				return nil
			}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicybinding %s: %v", mapBindingName, err)
		}
	}
	return nil
}

// deleteMAPVariants removes any MutatingAdmissionPolicy (+ binding) previously generated for this
// policy that isn't in the current desired set - mirrors deleteVAPVariants (generate-vap.go); see
// its doc comment for why ownership is checked by UID rather than the name prefix alone.
func (c *controller) deleteMAPVariants(ctx context.Context, desired []mapVariant, policy engineapi.GenericPolicy, baseName string, version admissionpolicy.MutatingAdmissionPolicyVersion) error {
	desiredNames := make(map[string]bool, len(desired))
	for _, v := range desired {
		desiredNames[v.name] = true
	}

	owned, err := c.listOwnedMAPNames(policy, baseName, version)
	if err != nil {
		return err
	}
	for _, name := range owned {
		if desiredNames[name] {
			continue
		}
		if err := c.deleteMAPByVersion(ctx, version, name, constructBindingName(name)); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) deleteMAPByVersion(ctx context.Context, version admissionpolicy.MutatingAdmissionPolicyVersion, name, bindingName string) error {
	switch version {
	case admissionpolicy.MutatingAdmissionPolicyVersionV1:
		if err := c.client.AdmissionregistrationV1().MutatingAdmissionPolicies().Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale mutatingadmissionpolicy %s: %v", name, err)
		}
		if err := c.client.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Delete(ctx, bindingName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale mutatingadmissionpolicybinding %s: %v", bindingName, err)
		}
	case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
		if err := c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale mutatingadmissionpolicy %s: %v", name, err)
		}
		if err := c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings().Delete(ctx, bindingName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale mutatingadmissionpolicybinding %s: %v", bindingName, err)
		}
	case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
		if err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale mutatingadmissionpolicy %s: %v", name, err)
		}
		if err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings().Delete(ctx, bindingName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale mutatingadmissionpolicybinding %s: %v", bindingName, err)
		}
	default:
		return fmt.Errorf("unsupported MutatingAdmissionPolicy version: %s", version)
	}
	return nil
}

// listOwnedMAPNames mirrors listOwnedVAPNames (generate-vap.go) for whichever MutatingAdmissionPolicy
// API version is in use.
func (c *controller) listOwnedMAPNames(policy engineapi.GenericPolicy, baseName string, version admissionpolicy.MutatingAdmissionPolicyVersion) ([]string, error) {
	uid := policy.GetUID()
	prefix := baseName + "-"
	matches := func(name string) bool { return name == baseName || strings.HasPrefix(name, prefix) }

	var names []string
	switch version {
	case admissionpolicy.MutatingAdmissionPolicyVersionV1:
		if c.mapV1Lister == nil {
			return nil, nil
		}
		all, err := c.mapV1Lister.List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list mutatingadmissionpolicies: %v", err)
		}
		for _, m := range all {
			if !matches(m.Name) {
				continue
			}
			for _, ref := range m.OwnerReferences {
				if ref.UID == uid {
					names = append(names, m.Name)
					break
				}
			}
		}
	case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
		if c.mapBetaLister == nil {
			return nil, nil
		}
		all, err := c.mapBetaLister.List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list mutatingadmissionpolicies: %v", err)
		}
		for _, m := range all {
			if !matches(m.Name) {
				continue
			}
			for _, ref := range m.OwnerReferences {
				if ref.UID == uid {
					names = append(names, m.Name)
					break
				}
			}
		}
	case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
		if c.mapAlphaLister == nil {
			return nil, nil
		}
		all, err := c.mapAlphaLister.List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list mutatingadmissionpolicies: %v", err)
		}
		for _, m := range all {
			if !matches(m.Name) {
				continue
			}
			for _, ref := range m.OwnerReferences {
				if ref.UID == uid {
					names = append(names, m.Name)
					break
				}
			}
		}
	}
	return names, nil
}
