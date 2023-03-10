package userinfo

import (
	"fmt"

	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	clusterroleKind = "ClusterRole"
	roleKind        = "Role"
)

type RoleBindingLister interface {
	List(labels.Selector) ([]*rbacv1.RoleBinding, error)
}

type ClusterRoleBindingLister interface {
	List(labels.Selector) ([]*rbacv1.ClusterRoleBinding, error)
}

// GetRoleRef gets the list of roles and cluster roles for the incoming api-request
func GetRoleRef(rbLister RoleBindingLister, crbLister ClusterRoleBindingLister, userInfo authenticationv1.UserInfo) ([]string, []string, error) {
	// rolebindings
	roleBindings, err := rbLister.List(labels.Everything())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list rolebindings: %v", err)
	}
	rs, crs := getRoleRefByRoleBindings(roleBindings, userInfo)
	// clusterrolebindings
	clusterroleBindings, err := crbLister.List(labels.Everything())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list clusterrolebindings: %v", err)
	}
	crs = append(crs, getRoleRefByClusterRoleBindings(clusterroleBindings, userInfo)...)
	if rs != nil {
		rs = sets.List(sets.New(rs...))
	}
	if crs != nil {
		crs = sets.List(sets.New(crs...))
	}
	return rs, crs, nil
}

func getRoleRefByRoleBindings(roleBindings []*rbacv1.RoleBinding, userInfo authenticationv1.UserInfo) ([]string, []string) {
	var roles, clusterRoles []string
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

func getRoleRefByClusterRoleBindings(clusterroleBindings []*rbacv1.ClusterRoleBinding, userInfo authenticationv1.UserInfo) []string {
	var clusterRoles []string
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
