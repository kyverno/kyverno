package admissionpolicygenerator

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// this file removes the admission policies Kyverno generated for a policy that another controller,
// such as Kyverno Authz, now owns. A policy can change owners when its evaluation mode is updated.
// The generated resources are deleted without writing the policy's status, since Kyverno no longer
// owns the policy.

// deleteFunc deletes an object by name.
type deleteFunc func(context.Context, string, metav1.DeleteOptions) error

// isOwnedBy reports whether obj has an owner reference to the policy with the given UID, which is
// how BuildValidatingAdmissionPolicy and BuildMutatingAdmissionPolicy mark the objects they generate.
func isOwnedBy(obj metav1.Object, owner types.UID) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == owner {
			return true
		}
	}
	return false
}

// deleteIfGenerated deletes the named object when get finds it in the informer cache and it was
// generated for the policy with the given UID. An object with the same name that Kyverno didn't
// generate for this policy is left alone. The delete is conditional on the cached object's UID, so
// a replacement created since the cache was filled isn't removed.
func deleteIfGenerated[T metav1.Object](ctx context.Context, name string, owner types.UID, get func(string) (T, error), del deleteFunc) error {
	obj, err := get(name)
	if err != nil {
		return nil
	}
	if !isOwnedBy(obj, owner) {
		logger.V(2).Info("skipping deletion of an admission policy resource that wasn't generated for this policy", "name", name)
		return nil
	}
	uid := obj.GetUID()
	if err := del(ctx, name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// deleteGeneratedVAP deletes the ValidatingAdmissionPolicy and binding generated for a ValidatingPolicy.
func (c *controller) deleteGeneratedVAP(ctx context.Context, policyName string, owner types.UID) error {
	vapName := "vpol-" + policyName
	if err := deleteIfGenerated(ctx, vapName, owner, c.getValidatingAdmissionPolicy, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
		return c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, name, opts)
	}); err != nil {
		return err
	}
	return deleteIfGenerated(ctx, constructBindingName(vapName), owner, c.getValidatingAdmissionPolicyBinding, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
		return c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, name, opts)
	})
}

// deleteGeneratedMAP deletes the MutatingAdmissionPolicy and binding generated for a MutatingPolicy,
// in every API version Kyverno has an informer for.
func (c *controller) deleteGeneratedMAP(ctx context.Context, policyName string, owner types.UID) error {
	mapName := "mpol-" + policyName
	bindingName := constructBindingName(mapName)
	deletes := []func() error{
		func() error {
			return deleteIfGenerated(ctx, mapName, owner, c.getMutatingAdmissionPolicyV1, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1().MutatingAdmissionPolicies().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfGenerated(ctx, bindingName, owner, c.getMutatingAdmissionPolicyBindingV1, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfGenerated(ctx, mapName, owner, c.getMutatingAdmissionPolicyBeta, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfGenerated(ctx, bindingName, owner, c.getMutatingAdmissionPolicyBindingBeta, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfGenerated(ctx, mapName, owner, c.getMutatingAdmissionPolicy, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfGenerated(ctx, bindingName, owner, c.getMutatingAdmissionPolicyBinding, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings().Delete(ctx, name, opts)
			})
		},
	}
	for _, del := range deletes {
		if err := del(); err != nil {
			return err
		}
	}
	return nil
}
