package admissionpolicygenerator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
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
				// When autogen is enabled, generate VAPs for each autogen config
				// instead of skipping VAP generation entirely.
				return c.handleVAPGenerationWithAutogen(ctx, pol, vapName, vapBindingName, observedVAP, vapErr, observedVAPbinding, vapBindingErr)
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
	c.eventGen.Add(event.NewValidatingAdmissionPolicyEvent(policy, observedVAP.Name, observedVAPbinding.Name)...)

	return nil
}

// handleVAPGenerationWithAutogen generates VAPs for each autogen config when pod controllers autogen is enabled.
// Instead of skipping VAP generation, it creates VAPs that target the pod controllers specified in the autogen config.
func (c *controller) handleVAPGenerationWithAutogen(
	ctx context.Context,
	pol *policiesv1beta1.ValidatingPolicy,
	baseVAPName string,
	baseVAPBindingName string,
	observedVAP *admissionregistrationv1.ValidatingAdmissionPolicy,
	vapErr error,
	observedVAPbinding *admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	vapBindingErr error,
) error {
	autogenConfigs := pol.GetStatus().Autogen.Configs

	// Sort the config keys for deterministic ordering
	configKeys := make([]string, 0, len(autogenConfigs))
	for key := range autogenConfigs {
		configKeys = append(configKeys, key)
	}
	sort.Strings(configKeys)

	// Track which VAPs we've generated so we can clean up any that are no longer needed
	generatedVAPNames := make(map[string]bool)

	for _, configKey := range configKeys {
		autogenConfig := autogenConfigs[configKey]
		vapName := baseVAPName + "-" + configKey
		vapBindingName := constructBindingName(vapName)
		generatedVAPNames[vapName] = true

		// Get or create the VAP for this autogen config
		autogenVAP, autogenVAPErr := c.getValidatingAdmissionPolicy(vapName)
		autogenVAPbinding, autogenVAPBindingErr := c.getValidatingAdmissionPolicyBinding(vapBindingName)

		// Create a copy of the policy with the autogen-generated spec
		autogenPolicy := pol.DeepCopy()
		autogenPolicy.Spec = *autogenConfig.Spec
		genericPolicy := engineapi.NewValidatingPolicy(autogenPolicy)

		celexceptions, err := c.getCELExceptions(pol.GetName())
		if err != nil {
			return fmt.Errorf("failed to get celexceptions by name %s: %v", pol.GetName(), err)
		}
		genericExceptions := make([]engineapi.GenericException, 0, len(celexceptions))
		for _, exception := range celexceptions {
			genericExceptions = append(genericExceptions, engineapi.NewCELPolicyException(&exception))
		}

		if autogenVAPErr != nil {
			if !apierrors.IsNotFound(autogenVAPErr) {
				return fmt.Errorf("failed to get validatingadmissionpolicy %s: %v", vapName, autogenVAPErr)
			}
			autogenVAP = &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: vapName,
				},
			}
		}
		if autogenVAPBindingErr != nil {
			if !apierrors.IsNotFound(autogenVAPBindingErr) {
				return fmt.Errorf("failed to get validatingadmissionpolicybinding %s: %v", vapBindingName, autogenVAPBindingErr)
			}
			autogenVAPbinding = &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: vapBindingName,
				},
			}
		}

		if autogenVAP.ResourceVersion == "" {
			err := admissionpolicy.BuildValidatingAdmissionPolicy(c.discoveryClient, autogenVAP, genericPolicy, genericExceptions)
			if err != nil {
				return fmt.Errorf("failed to build validatingadmissionpolicy %s: %v", autogenVAP.GetName(), err)
			}
			_, err = c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, autogenVAP, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create validatingadmissionpolicy %s: %v", autogenVAP.GetName(), err)
			}
		} else {
			_, err := controllerutils.Update(
				ctx,
				autogenVAP,
				c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies(),
				func(observed *admissionregistrationv1.ValidatingAdmissionPolicy) error {
					return admissionpolicy.BuildValidatingAdmissionPolicy(c.discoveryClient, observed, genericPolicy, genericExceptions)
				})
			if err != nil {
				return fmt.Errorf("failed to update validatingadmissionpolicy %s: %v", autogenVAP.GetName(), err)
			}
		}

		if autogenVAPbinding.ResourceVersion == "" {
			err := admissionpolicy.BuildValidatingAdmissionPolicyBinding(autogenVAPbinding, genericPolicy)
			if err != nil {
				return fmt.Errorf("failed to build validatingadmissionpolicybinding %s: %v", autogenVAPbinding.GetName(), err)
			}
			_, err = c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, autogenVAPbinding, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create validatingadmissionpolicybinding %s: %v", autogenVAPbinding.GetName(), err)
			}
		} else {
			_, err := controllerutils.Update(
				ctx,
				autogenVAPbinding,
				c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings(),
				func(observed *admissionregistrationv1.ValidatingAdmissionPolicyBinding) error {
					return admissionpolicy.BuildValidatingAdmissionPolicyBinding(observed, genericPolicy)
				})
			if err != nil {
				return fmt.Errorf("failed to update validatingadmissionpolicybinding %s: %v", autogenVAPbinding.GetName(), err)
			}
		}
	}

	// Clean up the base VAP (without autogen suffix) if it exists, since we now use autogen-specific VAPs
	if vapErr == nil {
		if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, baseVAPName, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	if vapBindingErr == nil {
		if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, baseVAPBindingName, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	// Clean up any autogen VAPs that are no longer needed (config was removed)
	existingVAPs, err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=kyverno",
	})
	if err == nil {
		for _, existingVAP := range existingVAPs.Items {
			if existingVAP.Name != baseVAPName && strings.HasPrefix(existingVAP.Name, baseVAPName+"-") {
				if !generatedVAPNames[existingVAP.Name] {
					if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, existingVAP.Name, metav1.DeleteOptions{}); err != nil {
						logger.Error(err, "failed to delete stale validatingadmissionpolicy", "name", existingVAP.Name)
					}
					// Also delete the corresponding binding
					bindingName := constructBindingName(existingVAP.Name)
					if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, bindingName, metav1.DeleteOptions{}); err != nil {
						logger.Error(err, "failed to delete stale validatingadmissionpolicybinding", "name", bindingName)
					}
				}
			}
		}
	}

	c.updatePolicyStatus(ctx, engineapi.NewValidatingPolicy(pol), true, "")
	c.eventGen.Add(event.NewValidatingAdmissionPolicyEvent(engineapi.NewValidatingPolicy(pol), baseVAPName, baseVAPBindingName)...)

	return nil
}