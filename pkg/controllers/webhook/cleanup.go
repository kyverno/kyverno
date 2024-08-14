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
)

// WebhookCleanupSetup creates temporary rbac owned by kyverno resources, these roles and cluster roles get automatically deleted when kyverno is uninstalled
// It creates the following resources:
//  1. Creates a temporary cluster role and cluster role binding with permission to delete kyverno's cluster role and set its owner ref to aggregated cluster role itself.
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
					Verbs:         []string{"get", "patch", "update"},
				},
			},
		}

		if cr, err := kubeClient.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "failed to create temporary clusterrole", "name", cr.Name)
			return err
		} else if !apierrors.IsAlreadyExists(err) {
			logger.V(4).Info("temporary clusterrole created", "clusterrole", cr.Name)
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

		if crb, err := kubeClient.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
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
					Verbs:         []string{"get", "create", "patch", "update", "delete"},
				},
				{
					APIGroups:     []string{"rbac.authorization.k8s.io"},
					Resources:     []string{"rolebindings", "roles"},
					ResourceNames: []string{name},
					Verbs:         []string{"get", "patch", "update"},
				},
				{
					APIGroups:     []string{"apps"},
					Resources:     []string{"deployments"},
					ResourceNames: []string{config.KyvernoDeploymentName()},
					Verbs:         []string{"get", "patch", "update"},
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
		if obj, err := AddFinalizers(ctx, kubeClient.RbacV1().ClusterRoleBindings(), name, finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to clusterrolebinding", "name", obj.GetName())
			return err
		}

		if obj, err := AddFinalizers(ctx, kubeClient.RbacV1().ClusterRoles(), name, finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to clusterrole", "name", obj.GetName())
			return err
		}

		if obj, err := AddFinalizers(ctx, kubeClient.RbacV1().ClusterRoles(), coreName, finalizer); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to add finalizer to clusterrole", "name", obj.GetName())
			return err
		}

		if obj, err := AddFinalizers(ctx, kubeClient.RbacV1().RoleBindings(config.KyvernoNamespace()), name, finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to rolebindings", "name", obj.GetName(), "namespace", obj.GetNamespace())
			return err
		}

		if obj, err := AddFinalizers(ctx, kubeClient.RbacV1().Roles(config.KyvernoNamespace()), name, finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to role", "name", obj.GetName(), "namespace", obj.GetNamespace())
			return err
		}

		if obj, err := AddFinalizers(ctx, kubeClient.AppsV1().Deployments(config.KyvernoNamespace()), config.KyvernoDeploymentName(), finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to deployment", "name", obj.GetName(), "namespace", obj.GetNamespace())
			return err
		}

		if obj, err := AddFinalizers(ctx, kubeClient.CoreV1().ServiceAccounts(config.KyvernoNamespace()), config.KyvernoServiceAccountName(), finalizer); err != nil {
			logger.Error(err, "failed to add finalizer to serviceaccount", "name", obj.GetName(), "namespace", obj.GetNamespace())
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
		if obj, err := DeleteFinalizers(ctx, kubeClient.RbacV1().ClusterRoleBindings(), name, finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from clusterrolebinding", "name", obj.GetName())
			return err
		}

		if obj, err := DeleteFinalizers(ctx, kubeClient.RbacV1().ClusterRoles(), name, finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from clusterrole", "name", obj.GetName())
			return err
		}

		if obj, err := DeleteFinalizers(ctx, kubeClient.RbacV1().ClusterRoles(), coreName, finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from clusterrole", "name", obj.GetName())
			return err
		}

		// cleanup namespace scoped rbac
		if obj, err := DeleteFinalizers(ctx, kubeClient.RbacV1().RoleBindings(config.KyvernoNamespace()), name, finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from rolebindings", "name", obj.GetName(), "namespace", obj.GetNamespace())
			return err
		}

		if obj, err := DeleteFinalizers(ctx, kubeClient.RbacV1().Roles(config.KyvernoNamespace()), name, finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from role", "name", obj.GetName(), "namespace", obj.GetNamespace())
			return err
		}

		if obj, err := DeleteFinalizers(ctx, kubeClient.AppsV1().Deployments(config.KyvernoNamespace()), config.KyvernoDeploymentName(), finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from deployment", "name", obj.GetName(), "namespace", obj.GetNamespace())
			return err
		}

		if obj, err := DeleteFinalizers(ctx, kubeClient.CoreV1().ServiceAccounts(config.KyvernoNamespace()), config.KyvernoDeploymentName(), finalizer); err != nil {
			logger.Error(err, "failed to delete finalizer from serviceaccount", "name", obj.GetName(), "namespace", obj.GetNamespace())
			return err
		}

		return nil
	}
}

func DeleteFinalizers[T metav1.Object](ctx context.Context, client controllerutils.ObjectClient[T], name, finalizer string) (T, error) {
	obj, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return obj, err
	}
	finalizers := make([]string, 0)
	for _, f := range obj.GetFinalizers() {
		if f != finalizer {
			finalizers = append(finalizers, f)
		}
	}

	obj.SetFinalizers(finalizers)
	obj, err = client.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return obj, err
	}
	return obj, nil
}

func AddFinalizers[T metav1.Object](ctx context.Context, client controllerutils.ObjectClient[T], name, finalizer string) (T, error) {
	obj, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return obj, err
	}

	finalizers := obj.GetFinalizers()
	finalizers = append(finalizers, finalizer)
	obj.SetFinalizers(finalizers)

	obj, err = client.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return obj, err
	}
	return obj, nil
}
