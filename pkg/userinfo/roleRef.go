package userinfo

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	engine "github.com/nirmata/kyverno/pkg/engine"
	v1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	rbaclister "k8s.io/client-go/listers/rbac/v1"
)

func GetRoleRef(rbLister rbaclister.RoleBindingLister, crbLister rbaclister.ClusterRoleBindingLister, request *v1beta1.AdmissionRequest) (roles []string, clusterRoles []string, err error) {
	// rolebindings
	roleBindings, err := rbLister.List(labels.NewSelector())
	if err != nil {
		return roles, clusterRoles, fmt.Errorf("failed to list rolebindings: %v", err)
	}

	rs, crs, err := getRoleRefByRoleBindings(roleBindings, request.UserInfo)
	if err != nil {
		return roles, clusterRoles, err
	}
	roles = append(roles, rs...)
	clusterRoles = append(clusterRoles, crs...)

	// clusterrolebindings
	clusterroleBindings, err := crbLister.List(labels.NewSelector())
	if err != nil {
		return roles, clusterRoles, fmt.Errorf("failed to list clusterrolebindings: %v", err)
	}

	crs, err = getRoleRefByClusterRoleBindings(clusterroleBindings, request.UserInfo)
	if err != nil {
		return roles, clusterRoles, err
	}
	clusterRoles = append(clusterRoles, crs...)

	return roles, clusterRoles, nil
}

func getRoleRefByRoleBindings(roleBindings []*rbacv1.RoleBinding, userInfo authenticationv1.UserInfo) (roles []string, clusterRoles []string, err error) {
	for _, rolebinding := range roleBindings {
		for _, subject := range rolebinding.Subjects {
			if !matchSubjectsMap(subject, userInfo) {
				continue
			}

			// roleRefMap := roleRef.(map[string]interface{})
			switch rolebinding.RoleRef.Kind {
			case "role":
				roles = append(roles, rolebinding.Namespace+":"+rolebinding.RoleRef.Name)
			case "clusterRole":
				clusterRoles = append(clusterRoles, rolebinding.RoleRef.Name)
			}
		}
	}

	return roles, clusterRoles, nil
}

// RoleRef in ClusterRoleBindings can only reference a ClusterRole in the global namespace
func getRoleRefByClusterRoleBindings(clusterroleBindings []*rbacv1.ClusterRoleBinding, userInfo authenticationv1.UserInfo) (clusterRoles []string, err error) {
	for _, clusterRoleBinding := range clusterroleBindings {
		for _, subject := range clusterRoleBinding.Subjects {
			if !matchSubjectsMap(subject, userInfo) {
				continue
			}

			if clusterRoleBinding.RoleRef.Kind == "clusterRole" {
				clusterRoles = append(clusterRoles, clusterRoleBinding.RoleRef.Name)
			}
		}
	}
	return clusterRoles, nil
}

// matchSubjectsMap checks if userInfo found in subject
// return true directly if found a match
// subject["kind"] can only be ServiceAccount, User and Group
func matchSubjectsMap(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	// ServiceAccount
	if isServiceaccountUserInfo(userInfo.Username) {
		return matchServiceAccount(subject, userInfo)
	}

	// User or Group
	return matchUserOrGroup(subject, userInfo)
}

func isServiceaccountUserInfo(username string) bool {
	if strings.Contains(username, engine.SaPrefix) {
		return true
	}
	return false
}

// matchServiceAccount checks if userInfo sa matche the subject sa
// serviceaccount represents as saPrefix:namespace:name in userInfo
func matchServiceAccount(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	subjectServiceAccount := subject.Namespace + ":" + subject.Name
	if userInfo.Username[len(engine.SaPrefix):] != subjectServiceAccount {
		glog.V(3).Infof("service account not match, expect %s, got %s", subjectServiceAccount, userInfo.Username[len(engine.SaPrefix):])
		return false
	}

	return true
}

// matchUserOrGroup checks if userInfo contains user or group info in a subject
func matchUserOrGroup(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	keys := append(userInfo.Groups, userInfo.Username)
	for _, key := range keys {
		if subject.Name == key {
			return true
		}
	}

	glog.V(3).Infof("user/group '%v' info not found in request userInfo: %v", subject.Name, keys)
	return false
}
