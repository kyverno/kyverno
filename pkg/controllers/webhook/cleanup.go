package webhook

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// WebhookCleanupSetup creates temporary rbac owned by kyverno resources, these roles and cluster roles get automatically deleted when kyverno is uninstalled
// It creates the following resources:
//  1. Creates a temporary cluster role and cluster role binding with permission to delete kyverno's cluster role and set its owner ref to aggregated cluster role itself.
//  2. Creates a temporary role and role binding with permissions to delete a service account, roles and role bindings with owner ref set to the service account.
func WebhookCleanupSetup(
	kubeClient kubernetes.Interface,
) func(context.Context, logr.Logger) error {
	return func(ctx context.Context, logger logr.Logger) error {
		name := config.KyvernoRoleName()
		coreName := name + ":core"
		tempRbacName := name + ":temporary"

		// create temporary rbac
		cr, err := kubeClient.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "failed to get cluster role binding")
			return err
		}

		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: tempRbacName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "rbac.authorization.k8s.io/v1",
						Kind:       "ClusterRole",
						Name:       cr.Name,
						UID:        cr.UID,
					},
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{"rbac.authorization.k8s.io"},
					Resources:     []string{"clusterrolebindings", "clusterroles"},
					ResourceNames: []string{name, coreName},
					Verbs:         []string{"patch"},
				},
			},
		}

		if cr, err := kubeClient.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{}); err != nil {
			logger.Error(err, "failed to create temporary clusterrole", "name", cr.Name)
			return err
		} else {
			logger.Info("temporary clusterrole created", "clusterrole", cr.Name)
		}

		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: tempRbacName,
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
				Name:     tempRbacName,
			},
		}

		if crb, err := kubeClient.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil {
			logger.Error(err, "failed to create temporary clusterrolebinding", "name", crb.Name)
			return err
		} else {
			logger.Info("temporary clusterrolebinding created", "clusterrolebinding", crb.Name)
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
					Verbs:         []string{"create", "patch", "update", "delete"},
				},
				{
					APIGroups:     []string{"rbac.authorization.k8s.io"},
					Resources:     []string{"rolebindings", "roles"},
					ResourceNames: []string{name},
					Verbs:         []string{"patch"},
				},
				{
					APIGroups:     []string{"apps"},
					Resources:     []string{"deployments"},
					ResourceNames: []string{config.KyvernoDeploymentName()},
					Verbs:         []string{"patch"},
				},
			},
		}

		if r, err := kubeClient.RbacV1().Roles(config.KyvernoNamespace()).Create(ctx, role, metav1.CreateOptions{}); err != nil {
			logger.Error(err, "failed to create temporary role", "name", r.Name)
			return err
		} else {
			logger.Info("temporary role created in kyverno namespace", "role", r.Name, "namespace", r.Namespace)
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

		if rb, err := kubeClient.RbacV1().RoleBindings(config.KyvernoNamespace()).Create(ctx, roleBinding, metav1.CreateOptions{}); err != nil {
			logger.Error(err, "failed to create temporary rolebinding", "name", rb.Name)
			return err
		} else {
			logger.Info("temporary rolebinding created in kyverno namespace", "rolebinding", rb.Name, "namespace", rb.Namespace)
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
// a. Removes finalizers from admission controller cluster role binding
// b. Removes finalizers from admission controller core cluster role
// c. Removes finalizers from admission controller aggregated cluster role
// d. Temporary cluster role and cluster role binding created by WebhookCleanupSetup gets garbage collected after (c) automatically
//
// Deletes the namespace scoped rbac in order:
// a. Removes finalizers from admission controller role binding.
// b. Removes finalizers from admission controller role.
// c. Removes finalizers from admission controller service account
// d. Temporary role and role binding created by WebhookCleanupSetup gets garbage collected after (c) automatically
func WebhookCleanupHandler(
	kubeClient kubernetes.Interface,
) func(context.Context, logr.Logger) error {
	return func(ctx context.Context, logger logr.Logger) error {
		finalizersRemovePatch := []byte(`[ { "op": "remove", "path": "/metadata/finalizers" } ]`)
		name := config.KyvernoRoleName()
		coreName := name + ":core"

		// cleanup cluster scoped rbac
		if crb, err := kubeClient.RbacV1().ClusterRoleBindings().Patch(ctx, name, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "already queued for deletion")
				return nil
			}
			logger.Error(err, "failed to patch clusterrolebindings")
			return err
		} else {
			logger.Info("finalizer removed from clusterrolebinding", "clusterrolebinding", crb.Name)
		}

		if cr, err := kubeClient.RbacV1().ClusterRoles().Patch(ctx, name, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "already queued for deletion")
				return nil
			}
			logger.Error(err, "failed to patch clusterrole")
			return err
		} else {
			logger.Info("finalizer removed from clusterrole", "clusterrole", cr.Name)
		}

		if cr, err := kubeClient.RbacV1().ClusterRoles().Patch(ctx, coreName, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "already queued for deletion")
				return nil
			}
			logger.Error(err, "failed to patch clusterrole")
			return err
		} else {
			logger.Info("finalizer removed from clusterrole", "clusterrole", cr.Name)
		}

		// cleanup namespace scoped rbac
		if rb, err := kubeClient.RbacV1().RoleBindings(config.KyvernoNamespace()).Patch(ctx, name, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "already queued for deletion")
				return nil
			}
			logger.Error(err, "failed to patch rolebinding")
			return err
		} else {
			logger.Info("finalizer removed from rolebinding", "rolebinding", rb.Name, "namespace", rb.Namespace)
		}

		if r, err := kubeClient.RbacV1().Roles(config.KyvernoNamespace()).Patch(ctx, name, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "already queued for deletion")
				return nil
			}
			logger.Error(err, "failed to patch role")
			return err
		} else {
			logger.Info("finalizer removed from role", "role", r.Name, "namespace", r.Namespace)
		}

		if d, err := kubeClient.AppsV1().Deployments(config.KyvernoNamespace()).Patch(ctx, config.KyvernoDeploymentName(), apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "already queued for deletion")
				return nil
			}
			logger.Error(err, "failed to patch role")
			return err
		} else {
			logger.Info("finalizer removed from kyverno deployment", "deployment", d.Name, "namespace", d.Namespace)
		}

		if sa, err := kubeClient.CoreV1().ServiceAccounts(config.KyvernoNamespace()).Patch(context.TODO(), config.KyvernoServiceAccountName(), apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "already queued for deletion")
				return nil
			}
			logger.Error(err, "failed to patch serviceaccount")
			return err
		} else {
			logger.Info("finalizer removed from serviceaccount", "serviceaccount", sa.Name, "namespace", sa.Namespace)
		}

		return nil
	}
}
