package userinfo

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/config"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

const (
	clusterroleKind = "ClusterRole"
	roleKind        = "Role"
)

// GetRoleRef gets the list of roles and cluster roles for the incoming api-request
func GetRoleRef(rbLister rbacv1listers.RoleBindingLister, crbLister rbacv1listers.ClusterRoleBindingLister, request *admissionv1.AdmissionRequest) ([]string, []string, error) {
	// rolebindings
	roleBindings, err := rbLister.List(labels.Everything())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list rolebindings: %v", err)
	}
	rs, crs := getRoleRefByRoleBindings(roleBindings, request.UserInfo)
	// clusterrolebindings
	clusterroleBindings, err := crbLister.List(labels.Everything())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list clusterrolebindings: %v", err)
	}
	crs = append(crs, getRoleRefByClusterRoleBindings(clusterroleBindings, request.UserInfo)...)
	return rs, crs, nil
}

func getRoleRefByRoleBindings(roleBindings []*rbacv1.RoleBinding, userInfo authenticationv1.UserInfo) (roles []string, clusterRoles []string) {
	for _, rolebinding := range roleBindings {
		if matchBindingSubjects(rolebinding.Subjects, userInfo, rolebinding.Namespace) {
			switch rolebinding.RoleRef.Kind {
			case roleKind:
				roles = append(roles, rolebinding.Namespace+":"+rolebinding.RoleRef.Name)
			case clusterroleKind:
				clusterRoles = append(clusterRoles, rolebinding.RoleRef.Name)
			}
		}
	}
	return roles, clusterRoles
}

func getRoleRefByClusterRoleBindings(clusterroleBindings []*rbacv1.ClusterRoleBinding, userInfo authenticationv1.UserInfo) (clusterRoles []string) {
	for _, clusterRoleBinding := range clusterroleBindings {
		if matchBindingSubjects(clusterRoleBinding.Subjects, userInfo, "") {
			if clusterRoleBinding.RoleRef.Kind == clusterroleKind {
				clusterRoles = append(clusterRoles, clusterRoleBinding.RoleRef.Name)
			}
		}
	}
	return clusterRoles
}

func matchBindingSubjects(subjects []rbacv1.Subject, userInfo authenticationv1.UserInfo, namespace string) bool {
	for _, subject := range subjects {
		switch subject.Kind {
		case rbacv1.ServiceAccountKind:
			ns := subject.Namespace
			if ns == "" {
				ns = namespace
			}
			if ns != "" {
				username := "system:serviceaccount:" + ns + ":" + subject.Name
				if userInfo.Username == username {
					return true
				}
			}
		case rbacv1.GroupKind:
			for _, group := range userInfo.Groups {
				if group == subject.Name {
					return true
				}
			}
		case rbacv1.UserKind:
			if userInfo.Username == subject.Name {
				return true
			}
		}
	}
	return false
}
