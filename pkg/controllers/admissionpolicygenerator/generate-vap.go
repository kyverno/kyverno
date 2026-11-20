package admissionpolicygenerator

import (
	"context"
	"errors"
	"fmt"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const validatingAdmissionPolicyKind = "ValidatingAdmissionPolicy"

func (c *controller) handleVAPGeneration(ctx context.Context, policy engineapi.GenericPolicy) (err error) {
	defer func() {
		if err != nil && c.queue != nil {
			c.queue.AddAfter("ValidatingPolicy/"+policy.GetName(), 10*time.Second)
		}
	}()
	reason, celexceptions, err := c.vapGenerationSkipReason(policy.AsValidatingPolicy())
	if err != nil {
		return c.handleGenerationError(ctx, policy, validatingAdmissionPolicyKind, err, nil)
	}
	if reason != "" {
		return c.removeValidatingAdmissionPolicy(ctx, policy, reason)
	}

	// Expiry is armed before these permission checks, which return an error so the workqueue retains a retry.
	// When generation or permission checks fail, also schedule a delayed retry to guarantee eventual recovery even if
	// the workqueue's immediate retry budget is exhausted.
	if !admissionpolicy.HasValidatingAdmissionPolicyPermission(c.checker) {
		logger.V(2).Info("insufficient permissions to generate ValidatingAdmissionPolicies")
		if err := c.updatePolicyStatus(ctx, policy, false, "insufficient permissions to generate ValidatingAdmissionPolicies"); err != nil {
			return err
		}
		return errors.New("insufficient permissions to generate ValidatingAdmissionPolicies")
	}
	if !admissionpolicy.HasValidatingAdmissionPolicyBindingPermission(c.checker) {
		logger.V(2).Info("insufficient permissions to generate ValidatingAdmissionPolicyBindings")
		if err := c.updatePolicyStatus(ctx, policy, false, "insufficient permissions to generate ValidatingAdmissionPolicyBindings"); err != nil {
			return err
		}
		return errors.New("insufficient permissions to generate ValidatingAdmissionPolicyBindings")
	}

	vapName, vapBindingName := validatingAdmissionPolicyNames(policy)
	if err := c.applyValidatingAdmissionPolicy(ctx, policy, vapName, vapBindingName, celexceptions); err != nil {
		return c.handleGenerationError(ctx, policy, validatingAdmissionPolicyKind, err, nil)
	}
	if err := c.updatePolicyStatus(ctx, policy, true, ""); err != nil {
		return err
	}
	c.eventGen.Add(event.NewValidatingAdmissionPolicyEvent(policy, vapName, vapBindingName)...)

	return nil
}

func validatingAdmissionPolicyNames(policy engineapi.GenericPolicy) (string, string) {
	vapName := "vpol-" + policy.GetName()
	return vapName, constructBindingName(vapName)
}

// vapGenerationSkipReason returns a non-empty reason when no ValidatingAdmissionPolicy may be
// generated for the policy, in which case any previously generated one is removed. Otherwise it
// returns the PolicyExceptions to translate into the ValidatingAdmissionPolicy.
func (c *controller) vapGenerationSkipReason(pol *policiesv1beta1.ValidatingPolicy) (string, []policiesv1beta1.PolicyException, error) {
	if !pol.GetSpec().GenerateValidatingAdmissionPolicyEnabled() {
		return "skip generating ValidatingAdmissionPolicy: not enabled.", nil, nil
	}
	// Read autogen from the spec: status.autogen is written later by the
	// policystatus controller and can still be empty on a new policy.
	autogenConfigs, err := vpolautogen.Autogen(pol)
	if err != nil {
		return "", nil, fmt.Errorf("failed to compute autogen configs for %s: %w", pol.GetName(), err)
	}
	if len(autogenConfigs) > 0 {
		return "skip generating ValidatingAdmissionPolicy: pod controllers autogen is enabled.", nil, nil
	}
	celexceptions, err := c.getCELExceptions(pol.GetName(), pol.GetKind())
	if err != nil {
		return "", nil, fmt.Errorf("failed to get celexceptions by name %s: %v", pol.GetName(), err)
	}
	c.requeueAtExpiry(pol.GetName(), "ValidatingPolicy", celexceptions)
	if ok, msg := admissionpolicy.CanGenerateNativePolicy(celexceptions); !ok {
		return "skip generating ValidatingAdmissionPolicy: " + msg, nil, nil
	}
	return "", celexceptions, nil
}

// removeValidatingAdmissionPolicy hands admission back to Kyverno, then deletes the generated
// ValidatingAdmissionPolicy and its binding. The status is written first because Kyverno skips the
// policy while status.generated is true: deleting first would leave nothing enforcing the policy if
// the status write then failed, and a failed delete would leave the stale native policy as the only
// enforcer.
func (c *controller) removeValidatingAdmissionPolicy(ctx context.Context, policy engineapi.GenericPolicy, reason string) error {
	if err := c.updatePolicyStatus(ctx, policy, false, reason); err != nil {
		return err
	}
	return deleteGenerated(ctx, c.generatedValidatingAdmissionPolicy(policy))
}

// generatedValidatingAdmissionPolicy returns the ValidatingAdmissionPolicy and binding in the
// informer cache that were generated for the policy.
func (c *controller) generatedValidatingAdmissionPolicy(policy engineapi.GenericPolicy) []generatedObject {
	vapName, vapBindingName := validatingAdmissionPolicyNames(policy)
	client := c.client.AdmissionregistrationV1()
	objs := appendIfGenerated(nil, policy, c.getValidatingAdmissionPolicy, vapName, client.ValidatingAdmissionPolicies().Delete)
	return appendIfGenerated(objs, policy, c.getValidatingAdmissionPolicyBinding, vapBindingName, client.ValidatingAdmissionPolicyBindings().Delete)
}

// applyValidatingAdmissionPolicy creates or updates the ValidatingAdmissionPolicy and its binding
// so that they match the policy.
func (c *controller) applyValidatingAdmissionPolicy(ctx context.Context, policy engineapi.GenericPolicy, vapName, vapBindingName string, celexceptions []policiesv1beta1.PolicyException) error {
	genericExceptions := make([]engineapi.GenericException, 0, len(celexceptions))
	for _, exception := range celexceptions {
		genericExceptions = append(genericExceptions, engineapi.NewCELPolicyException(&exception))
	}

	observedVAP, err := c.getValidatingAdmissionPolicy(vapName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get validatingadmissionpolicy %s: %v", vapName, err)
		}
		observedVAP = &admissionregistrationv1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: vapName,
			},
		}
	}
	observedVAPbinding, err := c.getValidatingAdmissionPolicyBinding(vapBindingName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get validatingadmissionpolicybinding %s: %v", vapBindingName, err)
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
	return nil
}
