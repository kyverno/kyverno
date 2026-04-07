package admissionpolicygenerator

import (
	"context"
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// preferredMAPVersion returns the API version to use based on which listers are initialised.
// Both the policy and binding listers must be set for a version to be considered available.
// v1beta1 takes precedence over v1alpha1 when both are fully available.
func (c *controller) preferredMAPVersion() (admissionpolicy.MutatingAdmissionPolicyVersion, bool) {
	if c.mapLister != nil && c.mapbindingLister != nil {
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
		return nil
	}
	logger.V(4).Info("Using MutatingAdmissionPolicy API", "version", version, "policy", mpol.GetName())
	return c.handleMAPGenerationWithVersion(ctx, mpol, version)
}

func (c *controller) handleMAPGenerationWithVersion(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, version admissionpolicy.MutatingAdmissionPolicyVersion) error {
	genericPolicy := engineapi.NewMutatingPolicy(mpol)
	if !admissionpolicy.HasMutatingAdmissionPolicyPermission(c.checker) {
		logger.V(2).Info("insufficient permissions to generate MutatingAdmissionPolicies")
		c.updatePolicyStatus(ctx, genericPolicy, false, "insufficient permissions to generate MutatingAdmissionPolicies")
		return nil
	}
	if !admissionpolicy.HasMutatingAdmissionPolicyBindingPermission(c.checker) {
		logger.V(2).Info("insufficient permissions to generate MutatingAdmissionPolicyBindings")
		c.updatePolicyStatus(ctx, genericPolicy, false, "insufficient permissions to generate MutatingAdmissionPolicyBindings")
		return nil
	}

	mapName := "mpol-" + mpol.GetName()
	mapBindingName := constructBindingName(mapName)

	wantMap := mpol.GetSpec().GenerateMutatingAdmissionPolicyEnabled()
	shouldDelete := !wantMap
	var reason string
	if wantMap {
		if len(mpol.GetStatus().Autogen.Configs) > 0 {
			shouldDelete = true
			reason = "skip generating MutatingAdmissionPolicy: pod controllers autogen is enabled."
		}
	} else {
		reason = "skip generating MutatingAdmissionPolicy: not enabled."
	}

	switch version {
	case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
		return c.handleMAPV1Beta1(ctx, mpol, mapName, mapBindingName, shouldDelete, reason, genericPolicy)
	case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
		return c.handleMAPV1Alpha1(ctx, mpol, mapName, mapBindingName, shouldDelete, reason, genericPolicy)
	default:
		return fmt.Errorf("unsupported MutatingAdmissionPolicy version: %s", version)
	}
}

func (c *controller) handleMAPV1Alpha1(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, mapName, mapBindingName string, shouldDelete bool, reason string, genericPolicy engineapi.GenericPolicy) error {
	observedMAP, mapErr := c.getMutatingAdmissionPolicy(mapName)
	observedMAPbinding, mapBindingErr := c.getMutatingAdmissionPolicyBinding(mapBindingName)

	if shouldDelete {
		if mapErr == nil {
			if err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Delete(ctx, mapName, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
		if mapBindingErr == nil {
			if err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings().Delete(ctx, mapBindingName, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
		c.updatePolicyStatus(ctx, genericPolicy, false, reason)
		return nil
	}

	celexceptions, err := c.getCELExceptions(mpol.GetName())
	if err != nil {
		return fmt.Errorf("failed to get celexceptions by name %s: %v", mpol.GetName(), err)
	}

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

	c.updatePolicyStatus(ctx, genericPolicy, true, "")
	return nil
}

func (c *controller) handleMAPV1Beta1(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy, mapName, mapBindingName string, shouldDelete bool, reason string, genericPolicy engineapi.GenericPolicy) error {
	observedMAP, mapErr := c.getMutatingAdmissionPolicyBeta(mapName)
	observedMAPbinding, mapBindingErr := c.getMutatingAdmissionPolicyBindingBeta(mapBindingName)

	if shouldDelete {
		if mapErr == nil {
			if err := c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Delete(ctx, mapName, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
		if mapBindingErr == nil {
			if err := c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings().Delete(ctx, mapBindingName, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
		c.updatePolicyStatus(ctx, genericPolicy, false, reason)
		return nil
	}

	celexceptions, err := c.getCELExceptions(mpol.GetName())
	if err != nil {
		return fmt.Errorf("failed to get celexceptions by name %s: %v", mpol.GetName(), err)
	}

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

	c.updatePolicyStatus(ctx, genericPolicy, true, "")
	return nil
}
