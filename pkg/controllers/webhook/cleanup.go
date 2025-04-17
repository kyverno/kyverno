package webhook

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// WebhookCleanupSetup creates temporary rbac owned by kyverno resources, these roles and cluster roles get automatically deleted when kyverno is uninstalled
// It creates the following resources:
//  1. Creates a temporary cluster role binding to give permission to delete kyverno's cluster role and set its owner ref to aggregated cluster role itself.
//  2. Creates a temporary role and role binding with permissions to delete a service account, roles and role bindings with owner ref set to the service account.
func WebhookCleanupSetup(
	kubeClient kubernetes.Interface,
	finalizer string,
) func(context.Context, logr.Logger) error {
	return func(ctx context.Context, logger logr.Logger) error {
		name := config.KyvernoRoleName()
		coreName := name + ":core"
		tempRbacName := name + ":temporary"

		// create temporary rbac
		cr, err := kubeClient.RbacV1().ClusterRoles().Get(ctx, coreName, metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "failed to get cluster role binding")
			return err
		}

		coreClusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: coreName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "rbac.authorization.k8s.io/v1",
						Kind:       "ClusterRole",
						Name:       cr.Name,
						UID:        cr.UID,
					},
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      config.KyvernoServiceAccountName(),
					Namespace: config.KyvernoNamespace(),
					APIGroup:  "",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     coreName,
			},
		}

		if crb, err := kubeClient.RbacV1().ClusterRoleBindings().Create(ctx, coreClusterRoleBinding, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "failed to create temporary clusterrolebinding", "name", crb.Name)
			return err
		} else if !apierrors.IsAlreadyExists(err) {
			logger.V(4).Info("temporary clusterrolebinding created", "clusterrolebinding", crb.Name)
		}

		// create temporary rbac
		sa, err := kubeClient.CoreV1().ServiceAccounts(config.KyvernoNamespace()).Get(ctx, config.KyvernoServiceAccountName(), metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "failed to get service account")
			return err
		}

		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tempRbacName,
				Namespace: config.KyvernoNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "ServiceAccount",
						Name:       sa.Name,
						UID:        sa.UID,
					},
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"serviceaccounts"},
					ResourceNames: []string{config.KyvernoServiceAccountName()},
					Verbs:         []string{"get", "update", "delete"},
				},
				{
					APIGroups:     []string{"rbac.authorization.k8s.io"},
					Resources:     []string{"rolebindings", "roles"},
					ResourceNames: []string{name},
					Verbs:         []string{"get", "update"},
				},
				{
					APIGroups:     []string{"apps"},
					Resources:     []string{"deployments"},
					ResourceNames: []string{config.KyvernoDeploymentName()},
					Verbs:         []string{"get", "update"},
				},
			},
		}

		if r, err := kubeClient.RbacV1().Roles(config.KyvernoNamespace()).Create(ctx, role, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "failed to create temporary role", "name", r.Name)
			return err
		} else if !apierrors.IsAlreadyExists(err) {
			logger.V(4).Info("temporary role created in kyverno namespace", "role", r.Name, "namespace", r.Namespace)
		}

		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tempRbacName,
				Namespace: config.KyvernoNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "ServiceAccount",
						Name:       sa.Name,
						UID:        sa.UID,
					},
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      config.KyvernoServiceAccountName(),
					Namespace: config.KyvernoNamespace(),
					APIGroup:  "",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     tempRbacName,
			},
		}

		if rb, err := kubeClient.RbacV1().RoleBindings(config.KyvernoNamespace()).Create(ctx, roleBinding, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "failed to create temporary rolebinding", "name", rb.Name)
			return err
		} else if !apierrors.IsAlreadyExists(err) {
			logger.V(4).Info("temporary rolebinding created in kyverno namespace", "rolebinding", rb.Name, "namespace", rb.Namespace)
		}

		// Add finalizers
		if err := AddFinalizers(ctx, kubeClient.RbacV1().ClusterRoles(), coreName, finalizer); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to add finalizer to clusterrole", "name", coreName)
			return err
		}

		if err := AddFinalizers(ctx, kubeClient.RbacV1().RoleBindings(config.KyvernoNamespace()), name, finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to rolebindings", "name", name, "namespace", config.KyvernoNamespace())
			return err
		}

		if err := AddFinalizers(ctx, kubeClient.RbacV1().Roles(config.KyvernoNamespace()), name, finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to role", "name", name, "namespace", config.KyvernoNamespace())
			return err
		}

		if err := AddFinalizers(ctx, kubeClient.AppsV1().Deployments(config.KyvernoNamespace()), config.KyvernoDeploymentName(), finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to deployment", "name", config.KyvernoDeploymentName(), "namespace", config.KyvernoNamespace())
			return err
		}

		if err := AddFinalizers(ctx, kubeClient.CoreV1().ServiceAccounts(config.KyvernoNamespace()), config.KyvernoServiceAccountName(), finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to serviceaccount", "name", config.KyvernoServiceAccountName(), "namespace", config.KyvernoNamespace())
			return err
		}
		return nil
	}
}

// WebhookCleanupHandler is run after webhook configuration cleanup is performed to delete roles and service account.
// Admission controller cluster and namespaced roles and role bindings have finalizers to block their deletion until admission controller terminates.
// This handler removes the finalizers on roles and service account after they are used to cleanup webhook cfg.
// It does the following:
//
// Deletes the cluster scoped rbac in order:
// a. Removes finalizers from controller cluster role binding
// b. Removes finalizers from controller core cluster role
// c. Removes finalizers from controller aggregated cluster role
// d. Temporary cluster role and cluster role binding created by WebhookCleanupSetup gets garbage collected after (c) automatically
//
// Deletes the namespace scoped rbac in order:
// a. Removes finalizers from controller role binding.
// b. Removes finalizers from controller role.
// c. Removes finalizers from controller service account
// d. Temporary role and role binding created by WebhookCleanupSetup gets garbage collected after (c) automatically
func WebhookCleanupHandler(
	kubeClient kubernetes.Interface,
	finalizer string,
) func(context.Context, logr.Logger) error {
	return func(ctx context.Context, logger logr.Logger) error {
		name := config.KyvernoRoleName()
		coreName := name + ":core"

		// cleanup cluster scoped rbac
		if err := DeleteFinalizers(ctx, kubeClient.RbacV1().ClusterRoles(), coreName, finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from clusterrole", "name", coreName)
			return err
		}

		// cleanup namespace scoped rbac
		if err := DeleteFinalizers(ctx, kubeClient.RbacV1().RoleBindings(config.KyvernoNamespace()), name, finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from rolebindings", "name", name, "namespace", config.KyvernoNamespace())
			return err
		}

		if err := DeleteFinalizers(ctx, kubeClient.RbacV1().Roles(config.KyvernoNamespace()), name, finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from role", "name", name, "namespace", config.KyvernoNamespace())
			return err
		}

		if err := DeleteFinalizers(ctx, kubeClient.AppsV1().Deployments(config.KyvernoNamespace()), config.KyvernoDeploymentName(), finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from deployment", "name", config.KyvernoDeploymentName(), "namespace", config.KyvernoNamespace())
			return err
		}

		if err := DeleteFinalizers(ctx, kubeClient.CoreV1().ServiceAccounts(config.KyvernoNamespace()), config.KyvernoServiceAccountName(), finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from serviceaccount", "name", config.KyvernoServiceAccountName(), "namespace", config.KyvernoNamespace())
			return err
		}

		return nil
	}
}

func DeleteFinalizers[T metav1.Object](ctx context.Context, client controllerutils.ObjectClient[T], name, finalizer string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		obj, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		finalizers := make([]string, 0)
		for _, f := range obj.GetFinalizers() {
			if f != finalizer {
				finalizers = append(finalizers, f)
			}
		}

		obj.SetFinalizers(finalizers)
		_, err = client.Update(ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
}

func AddFinalizers[T metav1.Object](ctx context.Context, client controllerutils.ObjectClient[T], name, finalizer string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		obj, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		finalizers := obj.GetFinalizers()
		for _, f := range finalizers {
			if f == finalizer {
				return nil
			}
		}
		finalizers = append(finalizers, finalizer)
		obj.SetFinalizers(finalizers)

		_, err = client.Update(ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
}
