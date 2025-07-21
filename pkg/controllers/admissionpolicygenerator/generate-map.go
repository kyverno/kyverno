package admissionpolicygenerator

import (
	"context"
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) handleMAPGeneration(ctx context.Context, mpol *policiesv1alpha1.MutatingPolicy) error {
	genericPolicy := engineapi.NewMutatingPolicy(mpol)
	// check if the controller has the required permissions to generate MutatingAdmissionPolicies.
	if !admissionpolicy.HasMutatingAdmissionPolicyPermission(c.checker) {
		logger.V(2).Info("insufficient permissions to generate MutatingAdmissionPolicies")
		c.updatePolicyStatus(ctx, genericPolicy, false, "insufficient permissions to generate MutatingAdmissionPolicies")
		return nil
	}
	// check if the controller has the required permissions to generate MutatingAdmissionPolicyBindings.
	if !admissionpolicy.HasMutatingAdmissionPolicyBindingPermission(c.checker) {
		logger.V(2).Info("insufficient permissions to generate MutatingAdmissionPolicyBindings")
		c.updatePolicyStatus(ctx, genericPolicy, false, "insufficient permissions to generate MutatingAdmissionPolicyBindings")
		return nil
	}

	mapName := "mpol-" + mpol.GetName()
	mapBindingName := constructBindingName(mapName)

	// get the MutatingAdmissionPolicy and MutatingAdmissionPolicyBinding if exists.
	observedMAP, mapErr := c.getMutatingAdmissionPolicy(mapName)
	observedMAPbinding, mapBindingErr := c.getMutatingAdmissionPolicyBinding(mapBindingName)
	wantMap := mpol.GetSpec().GenerateMutatingAdmissionPolicyEnabled()
	shouldDelete := !wantMap

	var reason string
	if wantMap {
		isAutogen := len(mpol.GetStatus().Autogen.Configs) > 0
		if isAutogen {
			shouldDelete = true
			reason = "skip generating MutatingAdmissionPolicy: pod controllers autogen is enabled."
		}
	} else {
		reason = "skip generating MutatingAdmissionPolicy: not enabled."
	}
	if shouldDelete {
		// delete the MutatingAdmissionPolicy if exist
		if mapErr == nil {
			if err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Delete(ctx, mapName, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
		// delete the MutatingAdmissionPolicyBinding if exist
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
			ObjectMeta: metav1.ObjectMeta{
				Name: mapName,
			},
		}
	}
	if mapBindingErr != nil {
		if !apierrors.IsNotFound(mapBindingErr) {
			return fmt.Errorf("failed to get mutatingadmissionpolicybinding %s: %v", mapBindingName, mapBindingErr)
		}
		observedMAPbinding = &admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: mapBindingName,
			},
		}
	}

	if observedMAP.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicy(observedMAP, mpol, celexceptions)
		_, err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Create(ctx, observedMAP, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicy %s: %v", observedMAP.GetName(), err)
		}
	} else {
		_, err := controllerutils.Update(
			ctx,
			observedMAP,
			c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies(),
			func(observed *admissionregistrationv1alpha1.MutatingAdmissionPolicy) error {
				admissionpolicy.BuildMutatingAdmissionPolicy(observedMAP, mpol, celexceptions)
				return nil
			})
		if err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicy %s: %v", observedMAP.GetName(), err)
		}
	}

	if observedMAPbinding.ResourceVersion == "" {
		admissionpolicy.BuildMutatingAdmissionPolicyBinding(observedMAPbinding, mpol)
		_, err := c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings().Create(ctx, observedMAPbinding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create mutatingadmissionpolicybinding %s: %v", observedMAPbinding.GetName(), err)
		}
	} else {
		_, err := controllerutils.Update(
			ctx,
			observedMAPbinding,
			c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings(),
			func(observed *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) error {
				admissionpolicy.BuildMutatingAdmissionPolicyBinding(observedMAPbinding, mpol)
				return nil
			})
		if err != nil {
			return fmt.Errorf("failed to update mutatingadmissionpolicybinding %s: %v", observedMAPbinding.GetName(), err)
		}
	}
	c.updatePolicyStatus(ctx, genericPolicy, true, "")
	return nil
}
