package admissionpolicygenerator

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// this file removes the admission policies Kyverno generated for a policy that another controller,
// such as Kyverno Authz, now owns. A policy can change owners when its evaluation mode is updated.
// The generated resources are deleted without writing the policy's status, since Kyverno no longer
// owns the policy.

// deleteIfExists deletes an object when get finds it in the informer cache.
func deleteIfExists[T any](ctx context.Context, name string, get func(string) (T, error), del func(context.Context, string, metav1.DeleteOptions) error) error {
	if _, err := get(name); err != nil {
		return nil
	}
	if err := del(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// deleteGeneratedVAP deletes the ValidatingAdmissionPolicy and binding generated for a ValidatingPolicy.
func (c *controller) deleteGeneratedVAP(ctx context.Context, policyName string) error {
	vapName := "vpol-" + policyName
	if err := deleteIfExists(ctx, vapName, c.getValidatingAdmissionPolicy, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
		return c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, name, opts)
	}); err != nil {
		return err
	}
	return deleteIfExists(ctx, constructBindingName(vapName), c.getValidatingAdmissionPolicyBinding, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
		return c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, name, opts)
	})
}

// deleteGeneratedMAP deletes the MutatingAdmissionPolicy and binding generated for a MutatingPolicy,
// in every API version Kyverno has an informer for.
func (c *controller) deleteGeneratedMAP(ctx context.Context, policyName string) error {
	mapName := "mpol-" + policyName
	bindingName := constructBindingName(mapName)
	deletes := []func() error{
		func() error {
			return deleteIfExists(ctx, mapName, c.getMutatingAdmissionPolicyV1, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1().MutatingAdmissionPolicies().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfExists(ctx, bindingName, c.getMutatingAdmissionPolicyBindingV1, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfExists(ctx, mapName, c.getMutatingAdmissionPolicyBeta, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfExists(ctx, bindingName, c.getMutatingAdmissionPolicyBindingBeta, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfExists(ctx, mapName, c.getMutatingAdmissionPolicy, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
				return c.client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Delete(ctx, name, opts)
			})
		},
		func() error {
			return deleteIfExists(ctx, bindingName, c.getMutatingAdmissionPolicyBinding, func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
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
