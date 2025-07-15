package admissionpolicygenerator

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

	var vapName string
	if polType == "ClusterPolicy" {
		vapName = "cpol-" + policy.GetName()
	} else {
		vapName = "vpol-" + policy.GetName()
	}
	vapBindingName := constructBindingName(vapName)
	// get the ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding if exists.
	observedVAP, vapErr := c.getValidatingAdmissionPolicy(vapName)
	observedVAPbinding, vapBindingErr := c.getValidatingAdmissionPolicyBinding(vapBindingName)

	genericExceptions := make([]engineapi.GenericException, 0)
	// in case of clusterpolicies, check if we can generate a VAP from it.
	if polType == "ClusterPolicy" {
		spec := policy.AsKyvernoPolicy().GetSpec()
		exceptions, err := c.getExceptions(policy.GetName(), spec.Rules[0].Name)
		if err != nil {
			return err
		}

		if ok, msg := admissionpolicy.CanGenerateVAP(spec, exceptions, false); !ok {
			// delete the ValidatingAdmissionPolicy if exist
			if vapErr == nil {
				err = c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, vapName, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
			// delete the ValidatingAdmissionPolicyBinding if exist
			if vapBindingErr == nil {
				err = c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, vapBindingName, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}

			if msg == "" {
				msg = "skip generating ValidatingAdmissionPolicy: a policy exception is configured."
			}
			c.updatePolicyStatus(ctx, policy, false, msg)
			return nil
		}
		for _, exception := range exceptions {
			genericExceptions = append(genericExceptions, engineapi.NewPolicyException(&exception))
		}
	} else {
		pol := policy.AsValidatingPolicy()
		wantVap := pol.GetSpec().GenerateValidatingAdmissionPolicyEnabled()
		shouldDelete := !wantVap

		var reason string
		if wantVap {
			isAutogen := len(pol.GetStatus().Autogen.Configs) > 0
			if isAutogen {
				shouldDelete = true
				reason = "skip generating ValidatingAdmissionPolicy: pod controllers autogen is enabled."
			}
		} else {
			reason = "skip generating ValidatingAdmissionPolicy: not enabled."
		}
		if shouldDelete {
			// delete the ValidatingAdmissionPolicy if exist
			if vapErr == nil {
				if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, vapName, metav1.DeleteOptions{}); err != nil {
					return err
				}
			}
			// delete the ValidatingAdmissionPolicyBinding if exist
			if vapBindingErr == nil {
				if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, vapBindingName, metav1.DeleteOptions{}); err != nil {
					return err
				}
			}
			c.updatePolicyStatus(ctx, policy, false, reason)
			return nil
		}
		celexceptions, err := c.getCELExceptions(policy.GetName())
		if err != nil {
			return fmt.Errorf("failed to get celexceptions by name %s: %v", policy.GetName(), err)
		}
		for _, exception := range celexceptions {
			genericExceptions = append(genericExceptions, engineapi.NewCELPolicyException(&exception))
		}
	}

	if vapErr != nil {
		if !apierrors.IsNotFound(vapErr) {
			return fmt.Errorf("failed to get validatingadmissionpolicy %s: %v", vapName, vapErr)
		}
		observedVAP = &admissionregistrationv1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: vapName,
			},
		}
	}
	if vapBindingErr != nil {
		if !apierrors.IsNotFound(vapBindingErr) {
			return fmt.Errorf("failed to get validatingadmissionpolicybinding %s: %v", vapBindingName, vapBindingErr)
		}
		observedVAPbinding = &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: vapBindingName,
			},
		}
	}

	if observedVAP.ResourceVersion == "" {
		err := admissionpolicy.BuildValidatingAdmissionPolicy(c.discoveryClient, observedVAP, policy, genericExceptions)
		if err != nil {
			return fmt.Errorf("failed to build validatingadmissionpolicy %s: %v", observedVAP.GetName(), err)
		}
		_, err = c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, observedVAP, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create validatingadmissionpolicy %s: %v", observedVAP.GetName(), err)
		}
	} else {
		_, err := controllerutils.Update(
			ctx,
			observedVAP,
			c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies(),
			func(observed *admissionregistrationv1.ValidatingAdmissionPolicy) error {
				return admissionpolicy.BuildValidatingAdmissionPolicy(c.discoveryClient, observed, policy, genericExceptions)
			})
		if err != nil {
			return fmt.Errorf("failed to update validatingadmissionpolicy %s: %v", observedVAP.GetName(), err)
		}
	}

	if observedVAPbinding.ResourceVersion == "" {
		err := admissionpolicy.BuildValidatingAdmissionPolicyBinding(observedVAPbinding, policy)
		if err != nil {
			return fmt.Errorf("failed to build validatingadmissionpolicybinding %s: %v", observedVAPbinding.GetName(), err)
		}
		_, err = c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, observedVAPbinding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create validatingadmissionpolicybinding %s: %v", observedVAPbinding.GetName(), err)
		}
	} else {
		_, err := controllerutils.Update(
			ctx,
			observedVAPbinding,
			c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings(),
			func(observed *admissionregistrationv1.ValidatingAdmissionPolicyBinding) error {
				return admissionpolicy.BuildValidatingAdmissionPolicyBinding(observed, policy)
			})
		if err != nil {
			return fmt.Errorf("failed to update validatingadmissionpolicybinding %s: %v", observedVAPbinding.GetName(), err)
		}
	}
	c.updatePolicyStatus(ctx, policy, true, "")
	// generate events
	e := event.NewValidatingAdmissionPolicyEvent(policy, observedVAP.Name, observedVAPbinding.Name)
	c.eventGen.Add(e...)
	return nil
}
