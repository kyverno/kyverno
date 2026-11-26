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

func (c *controller) handleVAPGeneration(ctx context.Context, policy engineapi.GenericPolicy) (err error) {
	if c.vapLister == nil || c.vapbindingLister == nil {
		logger.V(2).Info("No ValidatingAdmissionPolicy lister available, skipping VAP generation", "policy", policy.GetName())
		pol := policy.AsValidatingPolicy()
		if pol != nil && (pol.Status.Generated || pol.GetSpec().GenerateValidatingAdmissionPolicyEnabled()) {
			message := "skip generating ValidatingAdmissionPolicy: no ValidatingAdmissionPolicy API/informers are available."
			if pol.GetSpec().GenerateValidatingAdmissionPolicyEnabled() {
				message = "skip generating ValidatingAdmissionPolicy: requested but no ValidatingAdmissionPolicy API/informers are available."
			}
			if err := c.updatePolicyStatus(ctx, policy, false, message); err != nil {
				if c.queue != nil {
					c.queue.AddAfter("ValidatingPolicy/"+policy.GetName(), 10*time.Second)
				}
				return err
			}
			return nil
		}
		return nil
	}

	defer func() {
		if err != nil && c.queue != nil {
			c.queue.AddAfter("ValidatingPolicy/"+policy.GetName(), 10*time.Second)
		}
	}()
	pol := policy.AsValidatingPolicy()
	wantVap := pol.GetSpec().GenerateValidatingAdmissionPolicyEnabled()
	shouldDelete := !wantVap

	var celexceptions []policiesv1beta1.PolicyException
	var reason string
	if wantVap {
		// Read autogen from the spec: status.autogen is written later by the
		// policystatus controller and can still be empty on a new policy.
		autogenConfigs, err := vpolautogen.Autogen(pol)
		if err != nil {
			return fmt.Errorf("failed to compute autogen configs for %s: %w", pol.GetName(), err)
		}
		isAutogen := len(autogenConfigs) > 0
		if isAutogen {
			shouldDelete = true
			reason = "skip generating ValidatingAdmissionPolicy: pod controllers autogen is enabled."
		} else {
			celexceptions, err = c.getCELExceptions(policy.GetName(), pol.GetKind(), pol.GetNamespace())
			if err != nil {
				return fmt.Errorf("failed to get celexceptions by name %s: %v", policy.GetName(), err)
			}
			c.requeueAtExpiry(policy.GetName(), "ValidatingPolicy", celexceptions)
			if ok, msg := admissionpolicy.CanGenerateNativePolicy(celexceptions); !ok {
				shouldDelete = true
				reason = "skip generating ValidatingAdmissionPolicy: " + msg
			}
		}
	} else {
		reason = "skip generating ValidatingAdmissionPolicy: not enabled."
	}

	vapName := "vpol-" + policy.GetName()
	vapBindingName := constructBindingName(vapName)
	// get the ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding if exists.
	observedVAP, vapErr := c.getValidatingAdmissionPolicy(vapName)
	observedVAPbinding, vapBindingErr := c.getValidatingAdmissionPolicyBinding(vapBindingName)

	// Generated resources are cluster-scoped. A matching name alone does not
	// authorize Kyverno to delete or overwrite someone else's admission policy.
	// Check both resources before performing either deletion (or update).
	if vapErr == nil && !isOwnedByValidatingPolicy(observedVAP, policy) {
		return c.updatePolicyStatus(ctx, policy, false,
			fmt.Sprintf("cannot manage ValidatingAdmissionPolicy %s: name collision (owner UID does not match)", vapName))
	}
	if vapBindingErr == nil && !isOwnedByValidatingPolicy(observedVAPbinding, policy) {
		return c.updatePolicyStatus(ctx, policy, false,
			fmt.Sprintf("cannot manage ValidatingAdmissionPolicyBinding %s: name collision (owner UID does not match)", vapBindingName))
	}

	if shouldDelete {
		// delete the ValidatingAdmissionPolicy if it is still the observed object
		if vapErr == nil {
			options := deletionOptionsForObservedObject(observedVAP)
			if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, vapName, options); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		// delete the binding if it is still the observed object
		if vapBindingErr == nil {
			options := deletionOptionsForObservedObject(observedVAPbinding)
			if err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, vapBindingName, options); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		return c.updatePolicyStatus(ctx, policy, false, reason)
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
	genericExceptions := make([]engineapi.GenericException, 0)
	for _, exception := range celexceptions {
		genericExceptions = append(genericExceptions, engineapi.NewCELPolicyException(&exception))
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

	if err := c.updatePolicyStatus(ctx, policy, true, ""); err != nil {
		return err
	}
	c.eventGen.Add(event.NewValidatingAdmissionPolicyEvent(policy, observedVAP.Name, observedVAPbinding.Name)...)

	return nil
}

// isOwnedByValidatingPolicy requires a matching nonempty UID, not just the
// generated resource name. An absent or foreign owner is never safe to mutate.
func isOwnedByValidatingPolicy(resource metav1.Object, policy engineapi.GenericPolicy) bool {
	if resource == nil || policy.GetUID() == "" {
		return false
	}
	for _, owner := range resource.GetOwnerReferences() {
		if owner.UID == policy.GetUID() && owner.Name == policy.GetName() && owner.Kind == "ValidatingPolicy" {
			return true
		}
	}
	return false
}

// deletionOptionsForObservedObject prevents a newly recreated object with the
// same name from being deleted using an old informer cache entry.
func deletionOptionsForObservedObject(resource metav1.Object) metav1.DeleteOptions {
	options := metav1.DeleteOptions{}
	if uid := resource.GetUID(); uid != "" {
		options.Preconditions = &metav1.Preconditions{UID: &uid}
	}
	return options
}
