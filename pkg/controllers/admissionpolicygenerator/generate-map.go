package admissionpolicygenerator

import (
	"context"
	"errors"
	"fmt"
	"time"

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
)

const mutatingAdmissionPolicyKind = "MutatingAdmissionPolicy"

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
		// A policy still marked generated has no MutatingAdmissionPolicy the cluster can serve any more.
		if mpol.GetSpec().GenerateMutatingAdmissionPolicyEnabled() || mpol.GetStatus().Generated {
			genericPolicy := engineapi.NewMutatingPolicy(mpol)
			if err := c.updatePolicyStatus(ctx, genericPolicy, false, "skip generating MutatingAdmissionPolicy: requested but no MutatingAdmissionPolicy API/informers are available."); err != nil {
				if c.queue != nil {
					c.queue.AddAfter("MutatingPolicy/"+mpol.GetName(), 10*time.Second)
				}
				return err
			}
		}
		return nil
	}
	logger.V(4).Info("Using MutatingAdmissionPolicy API", "version", version, "policy", mpol.GetName())
	return c.handleMAPGenerationWithVersion(ctx, mpol, version)
}

func (c *controller) handleMAPGenerationWithVersion(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, version admissionpolicy.MutatingAdmissionPolicyVersion) (err error) {
	defer func() {
		if err != nil && c.queue != nil {
			c.queue.AddAfter("MutatingPolicy/"+mpol.GetName(), 10*time.Second)
		}
	}()
	genericPolicy := engineapi.NewMutatingPolicy(mpol)
	removeStaleMAP := func(ctx context.Context) error {
		return deleteGenerated(ctx, c.generatedMutatingAdmissionPolicy(mpol, version))
	}

	reason, err := mapGenerationSkipReason(mpol)
	if err != nil {
		return c.handleGenerationError(ctx, genericPolicy, mutatingAdmissionPolicyKind, fmt.Errorf("failed to compute autogen configs for %s: %w", mpol.GetName(), err), removeStaleMAP)
	}
	var celexceptions []policiesv1beta1.PolicyException
	if reason == "" {
		celexceptions, err = c.getCELExceptions(mpol.GetName(), mpol.GetKind())
		if err != nil {
			return c.handleGenerationError(ctx, genericPolicy, mutatingAdmissionPolicyKind, fmt.Errorf("failed to get celexceptions by name %s: %v", mpol.GetName(), err), removeStaleMAP)
		}
		c.requeueAtExpiry(mpol.GetName(), "MutatingPolicy", celexceptions)
		if ok, msg := admissionpolicy.CanGenerateNativeMutatingPolicy(celexceptions); !ok {
			reason = "skip generating MutatingAdmissionPolicy: " + msg
		}
	}
	if reason != "" {
		return c.removeMutatingAdmissionPolicy(ctx, mpol, version, reason)
	}

	// Without these permissions the MutatingAdmissionPolicy can no longer be kept in step with the
	// policy, so it is removed as for any other reason to stop generating it. Expiry is armed before
	// these checks, and the error keeps a retry queued until the permissions are back.
	missingPermission := func(msg string) error {
		logger.V(2).Info(msg)
		if err := c.removeMutatingAdmissionPolicy(ctx, mpol, version, msg); err != nil {
			return err
		}
		return errors.New(msg)
	}
	if !admissionpolicy.HasMutatingAdmissionPolicyPermissionForVersion(version, c.checker) {
		return missingPermission("insufficient permissions to generate MutatingAdmissionPolicies")
	}
	if !admissionpolicy.HasMutatingAdmissionPolicyBindingPermissionForVersion(version, c.checker) {
		return missingPermission("insufficient permissions to generate MutatingAdmissionPolicyBindings")
	}

	if err := c.handleMAPVersion(ctx, mpol, version, celexceptions); err != nil {
		return c.handleGenerationError(ctx, genericPolicy, mutatingAdmissionPolicyKind, err, removeStaleMAP)
	}
	return c.updatePolicyStatus(ctx, genericPolicy, true, "")
}

func mutatingAdmissionPolicyNames(mpol *policiesv1beta1.MutatingPolicy) (string, string) {
	mapName := "mpol-" + mpol.GetName()
	return mapName, constructBindingName(mapName)
}

// removeMutatingAdmissionPolicy hands admission back to Kyverno, then deletes the generated
// MutatingAdmissionPolicy and its binding, in that order for the reasons given on
// removeValidatingAdmissionPolicy. A failed delete is returned so that the workqueue retries it.
func (c *controller) removeMutatingAdmissionPolicy(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, version admissionpolicy.MutatingAdmissionPolicyVersion, reason string) error {
	if err := c.updatePolicyStatus(ctx, engineapi.NewMutatingPolicy(mpol), false, reason); err != nil {
		return err
	}
	return deleteGenerated(ctx, c.generatedMutatingAdmissionPolicy(mpol, version))
}

// hasGeneratedMutatingAdmissionPolicy reports whether the informer cache still holds a
// MutatingAdmissionPolicy or binding generated for the policy.
func (c *controller) hasGeneratedMutatingAdmissionPolicy(mpol *policiesv1beta1.MutatingPolicy) bool {
	version, ok := c.preferredMAPVersion()
	return ok && len(c.generatedMutatingAdmissionPolicy(mpol, version)) > 0
}

// generatedMutatingAdmissionPolicy returns the MutatingAdmissionPolicy and binding of the given API
// version in the informer cache that were generated for the policy.
func (c *controller) generatedMutatingAdmissionPolicy(mpol *policiesv1beta1.MutatingPolicy, version admissionpolicy.MutatingAdmissionPolicyVersion) []generatedObject {
	mapName, mapBindingName := mutatingAdmissionPolicyNames(mpol)
	switch version {
	case admissionpolicy.MutatingAdmissionPolicyVersionV1:
		client := c.client.AdmissionregistrationV1()
		objs := appendIfGenerated(nil, mpol, c.getMutatingAdmissionPolicyV1, mapName, client.MutatingAdmissionPolicies().Delete)
		return appendIfGenerated(objs, mpol, c.getMutatingAdmissionPolicyBindingV1, mapBindingName, client.MutatingAdmissionPolicyBindings().Delete)
	case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
		client := c.client.AdmissionregistrationV1beta1()
		objs := appendIfGenerated(nil, mpol, c.getMutatingAdmissionPolicyBeta, mapName, client.MutatingAdmissionPolicies().Delete)
		return appendIfGenerated(objs, mpol, c.getMutatingAdmissionPolicyBindingBeta, mapBindingName, client.MutatingAdmissionPolicyBindings().Delete)
	case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
		client := c.client.AdmissionregistrationV1alpha1()
		objs := appendIfGenerated(nil, mpol, c.getMutatingAdmissionPolicy, mapName, client.MutatingAdmissionPolicies().Delete)
		return appendIfGenerated(objs, mpol, c.getMutatingAdmissionPolicyBinding, mapBindingName, client.MutatingAdmissionPolicyBindings().Delete)
	}
	return nil
}

// handleMAPVersion creates or updates the MutatingAdmissionPolicy and binding of the given API
// version so that they match the policy.
func (c *controller) handleMAPVersion(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, version admissionpolicy.MutatingAdmissionPolicyVersion, celexceptions []policiesv1beta1.PolicyException) error {
	mapName, mapBindingName := mutatingAdmissionPolicyNames(mpol)
	switch version {
	case admissionpolicy.MutatingAdmissionPolicyVersionV1:
		return c.handleMAPV1(ctx, mpol, mapName, mapBindingName, celexceptions)
	case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
		return c.handleMAPV1Beta1(ctx, mpol, mapName, mapBindingName, celexceptions)
	case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
		return c.handleMAPV1Alpha1(ctx, mpol, mapName, mapBindingName, celexceptions)
	default:
		return fmt.Errorf("unsupported MutatingAdmissionPolicy version: %s", version)
	}
}

func (c *controller) handleMAPV1(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, mapName, mapBindingName string, celexceptions []policiesv1beta1.PolicyException) error {
	observedMAP, mapErr := c.getMutatingAdmissionPolicyV1(mapName)
	observedMAPbinding, mapBindingErr := c.getMutatingAdmissionPolicyBindingV1(mapBindingName)

	if mapErr != nil {
		if !apierrors.IsNotFound(mapErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicy %s: %v", mapName, mapErr)
		}
		observedMAP = &admissionregistrationv1.MutatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: mapName},
		}
	}
	if mapBindingErr != nil {
		if !apierrors.IsNotFound(mapBindingErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicybinding %s: %v", mapBindingName, mapBindingErr)
		}
		observedMAPbinding = &admissionregistrationv1.MutatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: mapBindingName},
		}
	}

	if observedMAP.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyV1(observedMAP, mpol, celexceptions)
		if _, err := c.client.AdmissionregistrationV1().MutatingAdmissionPolicies().Create(ctx, observedMAP, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicy %s: %v", observedMAP.GetName(), err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAP, c.client.AdmissionregistrationV1().MutatingAdmissionPolicies(), func(observed *admissionregistrationv1.MutatingAdmissionPolicy) error {
			admissionpolicy.BuildMutatingAdmissionPolicyV1(observed, mpol, celexceptions)
			return nil
		}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicy %s: %v", observedMAP.GetName(), err)
		}
	}

	if observedMAPbinding.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyBindingV1(observedMAPbinding, mpol)
		if _, err := c.client.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Create(ctx, observedMAPbinding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicybinding %s: %v", observedMAPbinding.GetName(), err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAPbinding, c.client.AdmissionregistrationV1().MutatingAdmissionPolicyBindings(), func(observed *admissionregistrationv1.MutatingAdmissionPolicyBinding) error {
			admissionpolicy.BuildMutatingAdmissionPolicyBindingV1(observed, mpol)
			return nil
		}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicybinding %s: %v", observedMAPbinding.GetName(), err)
		}
	}

	return nil
}

// mapGenerationSkipReason returns a non-empty reason when a MutatingAdmissionPolicy must not be
// generated for the policy, in which case any previously generated policy is deleted instead.
// useServerSideApply mutates atomic fields that a native MutatingAdmissionPolicy rejects, so a
// generated MAP (which becomes the sole admission path once status.generated is set) would drop the
// mutation. Pod-controller autogen is likewise incompatible with MAP generation.
func mapGenerationSkipReason(mpol *policiesv1beta1.MutatingPolicy) (string, error) {
	if !mpol.GetSpec().GenerateMutatingAdmissionPolicyEnabled() {
		return "skip generating MutatingAdmissionPolicy: not enabled.", nil
	}
	if ec := mpol.GetSpec().EvaluationConfiguration; ec != nil && ec.UseServerSideApply {
		return "skip generating MutatingAdmissionPolicy: useServerSideApply is enabled, which mutates atomic fields that a native MutatingAdmissionPolicy rejects.", nil
	}
	// Read autogen from the spec: status.autogen is written later by the
	// policystatus controller and can still be empty on a new policy.
	autogenConfigs, err := mpolautogen.Autogen(mpol)
	if err != nil {
		return "", err
	}
	if len(autogenConfigs) > 0 {
		return "skip generating MutatingAdmissionPolicy: pod controllers autogen is enabled.", nil
	}
	return "", nil
}

func (c *controller) handleMAPV1Alpha1(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, mapName, mapBindingName string, celexceptions []policiesv1beta1.PolicyException) error {
	observedMAP, mapErr := c.getMutatingAdmissionPolicy(mapName)
	observedMAPbinding, mapBindingErr := c.getMutatingAdmissionPolicyBinding(mapBindingName)

	if mapErr != nil {
		if !apierrors.IsNotFound(mapErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicy %s: %v", mapName, mapErr)
		}
		observedMAP = &admissionregistrationv1alpha1.MutatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: mapName},
		}
	}
	if mapBindingErr != nil {
		if !apierrors.IsNotFound(mapBindingErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicybinding %s: %v", mapBindingName, mapBindingErr)
		}
		observedMAPbinding = &admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: mapBindingName},
		}
	}

	if observedMAP.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicy(observedMAP, mpol, celexceptions)
		if _, err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Create(ctx, observedMAP, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicy %s: %v", observedMAP.GetName(), err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAP, c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies(),
			func(observed *admissionregistrationv1alpha1.MutatingAdmissionPolicy) error {
				admissionpolicy.BuildMutatingAdmissionPolicy(observed, mpol, celexceptions)
				return nil
			}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicy %s: %v", observedMAP.GetName(), err)
		}
	}

	if observedMAPbinding.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyBinding(observedMAPbinding, mpol)
		if _, err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings().Create(ctx, observedMAPbinding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicybinding %s: %v", observedMAPbinding.GetName(), err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAPbinding, c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings(),
			func(observed *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) error {
				admissionpolicy.BuildMutatingAdmissionPolicyBinding(observed, mpol)
				return nil
			}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicybinding %s: %v", observedMAPbinding.GetName(), err)
		}
	}

	return nil
}

func (c *controller) handleMAPV1Beta1(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, mapName, mapBindingName string, celexceptions []policiesv1beta1.PolicyException) error {
	observedMAP, mapErr := c.getMutatingAdmissionPolicyBeta(mapName)
	observedMAPbinding, mapBindingErr := c.getMutatingAdmissionPolicyBindingBeta(mapBindingName)

	if mapErr != nil {
		if !apierrors.IsNotFound(mapErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicy %s: %v", mapName, mapErr)
		}
		observedMAP = &admissionregistrationv1beta1.MutatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: mapName},
		}
	}
	if mapBindingErr != nil {
		if !apierrors.IsNotFound(mapBindingErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicybinding %s: %v", mapBindingName, mapBindingErr)
		}
		observedMAPbinding = &admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: mapBindingName},
		}
	}

	if observedMAP.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyBeta(observedMAP, mpol, celexceptions)
		if _, err := c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Create(ctx, observedMAP, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicy %s: %v", observedMAP.GetName(), err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAP, c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicies(),
			func(observed *admissionregistrationv1beta1.MutatingAdmissionPolicy) error {
				admissionpolicy.BuildMutatingAdmissionPolicyBeta(observed, mpol, celexceptions)
				return nil
			}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicy %s: %v", observedMAP.GetName(), err)
		}
	}

	if observedMAPbinding.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyBindingBeta(observedMAPbinding, mpol)
		if _, err := c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings().Create(ctx, observedMAPbinding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicybinding %s: %v", observedMAPbinding.GetName(), err)
		}
	} else {
		if _, err := controllerutils.Update(ctx, observedMAPbinding, c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings(),
			func(observed *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding) error {
				admissionpolicy.BuildMutatingAdmissionPolicyBindingBeta(observed, mpol)
				return nil
			}); err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicybinding %s: %v", observedMAPbinding.GetName(), err)
		}
	}

	return nil
}
