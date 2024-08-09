package webhooks

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// PostWebhookCleanupHandler is run after webhook configuration cleanup is performed to delete roles and service account.
// Admission controller cluster and namespaced roles and role bindings have finalizers to block their deletion until admission controller terminates.
// This handler removes the finalizers on roles and service account after they are used to cleanup webhook cfg.
// It does the following:
//  1. Creates a temporary cluster role and cluster role binding with permission to delete kyverno's cluster role and set its owner ref to aggregated cluster role itself.
//  2. Deletes the cluster scoped rbac in order:
//     a. Removes finalizers from admission controller cluster role binding
//     b. Removes finalizers from admission controller core cluster role
//     c. Removes finalizers from admission controller aggregated cluster role
//     d. Temporary cluster role and cluster role binding gets garbage collected after (c) automatically
//  3. Creates a temporary role and role binding with permissions to delete a service account, roles and role bindings with owner ref set to the service account.
//  4. Deletes the namespace scoped rbac in order:
//     a. Removes finalizers from admission controller role binding.
//     b. Removes finalizers from admission controller role.
//     c. Removes finalizers from admission controller service account
//     d. Temporary role and role binding gets garbage collected after (c) automatically
func PostWebhookCleanupHandler(
	logger logr.Logger,
	kubeClient kubernetes.Interface,
) func(context.Context) {
	return func(ctx context.Context) {
		finalizersRemovePatch := []byte(`[ { "op": "remove", "path": "/metadata/finalizers" } ]`)
		name := config.KyvernoRoleName()
		coreName := name + ":core"
		tempRbacName := name + ":temporary"

		// create temporary rbac
		cr, err := kubeClient.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "failed to get cluster role binding")
			return
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
			return
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
			return
		} else {
			logger.Info("temporary clusterrolebinding created", "clusterrolebinding", crb.Name)
		}

		// cleanup cluster scoped rbac
		if crb, err := kubeClient.RbacV1().ClusterRoleBindings().Patch(ctx, name, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			logger.Error(err, "failed to patch clusterrolebindings")
			return
		} else {
			logger.Info("finalizer removed from clusterrolebinding", "clusterrolebinding", crb.Name)
		}

		if cr, err := kubeClient.RbacV1().ClusterRoles().Patch(ctx, name, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			logger.Error(err, "failed to patch clusterrole")
			return
		} else {
			logger.Info("finalizer removed from clusterrole", "clusterrole", cr.Name)
		}

		if cr, err := kubeClient.RbacV1().ClusterRoles().Patch(ctx, coreName, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			logger.Error(err, "failed to patch clusterrole")
			return
		} else {
			logger.Info("finalizer removed from clusterrole", "clusterrole", cr.Name)
		}

		// create temporary rbac
		sa, err := kubeClient.CoreV1().ServiceAccounts(config.KyvernoNamespace()).Get(ctx, config.KyvernoServiceAccountName(), metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "failed to get service account")
			return
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
			},
		}

		if r, err := kubeClient.RbacV1().Roles(config.KyvernoNamespace()).Create(ctx, role, metav1.CreateOptions{}); err != nil {
			logger.Error(err, "failed to create temporary role", "name", r.Name)
			return
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
			return
		} else {
			logger.Info("temporary rolebinding created in kyverno namespace", "rolebinding", rb.Name, "namespace", rb.Namespace)
		}

		// cleanup namespace scoped rbac
		if rb, err := kubeClient.RbacV1().RoleBindings(config.KyvernoNamespace()).Patch(ctx, name, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			logger.Error(err, "failed to patch rolebinding")
			return
		} else {
			logger.Info("finalizer removed from rolebinding", "rolebinding", rb.Name, "namespace", rb.Namespace)
		}

		if r, err := kubeClient.RbacV1().Roles(config.KyvernoNamespace()).Patch(ctx, name, apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			logger.Error(err, "failed to patch role")
			return
		} else {
			logger.Info("finalizer removed from role", "role", r.Name, "namespace", r.Namespace)
		}

		if sa, err := kubeClient.CoreV1().ServiceAccounts(config.KyvernoNamespace()).Patch(ctx, config.KyvernoServiceAccountName(), apimachinerytypes.JSONPatchType, finalizersRemovePatch, metav1.PatchOptions{}); err != nil {
			logger.Error(err, "failed to patch serviceaccount")
			return
		} else {
			logger.Info("finalizer removed from serviceaccount", "serviceaccount", sa.Name, "namespace", sa.Namespace)
		}
	}
}
