package userinfo

import (
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

const (
	clusterroleKind = "ClusterRole"
	roleKind        = "Role"
	// saPrefix represents service account prefix in admission requests
	saPrefix = "system:serviceaccount:"
)

// GetRoleRef gets the list of roles and cluster roles for the incoming api-request
func GetRoleRef(rbLister rbacv1listers.RoleBindingLister, crbLister rbacv1listers.ClusterRoleBindingLister, request *admissionv1.AdmissionRequest, dynamicConfig config.Configuration) ([]string, []string, error) {
	keys := append(request.UserInfo.Groups, request.UserInfo.Username)
	if utils.SliceContains(keys, dynamicConfig.GetExcludeGroupRole()...) {
		return nil, nil, nil
	}
	// rolebindings
	roleBindings, err := rbLister.List(labels.Everything())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list rolebindings: %v", err)
	}
	rs, crs := getRoleRefByRoleBindings(roleBindings, request.UserInfo)
	// clusterrolebindings
	clusterroleBindings, err := crbLister.List(labels.NewSelector())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list clusterrolebindings: %v", err)
	}
	crs = append(crs, getRoleRefByClusterRoleBindings(clusterroleBindings, request.UserInfo)...)
	return rs, crs, nil
}

func getRoleRefByRoleBindings(roleBindings []*rbacv1.RoleBinding, userInfo authenticationv1.UserInfo) (roles []string, clusterRoles []string) {
	for _, rolebinding := range roleBindings {
		for _, subject := range rolebinding.Subjects {
			if matchSubjectsMap(subject, userInfo, rolebinding.Namespace) {
				switch rolebinding.RoleRef.Kind {
				case roleKind:
					roles = append(roles, rolebinding.Namespace+":"+rolebinding.RoleRef.Name)
				case clusterroleKind:
					clusterRoles = append(clusterRoles, rolebinding.RoleRef.Name)
				}
			}
		}
	}
	return roles, clusterRoles
}

// RoleRef in ClusterRoleBindings can only reference a ClusterRole in the global namespace
func getRoleRefByClusterRoleBindings(clusterroleBindings []*rbacv1.ClusterRoleBinding, userInfo authenticationv1.UserInfo) (clusterRoles []string) {
	for _, clusterRoleBinding := range clusterroleBindings {
		for _, subject := range clusterRoleBinding.Subjects {
			if matchSubjectsMap(subject, userInfo, subject.Namespace) {
				if clusterRoleBinding.RoleRef.Kind == clusterroleKind {
					clusterRoles = append(clusterRoles, clusterRoleBinding.RoleRef.Name)
				}
			}
		}
	}
	return clusterRoles
}

// matchSubjectsMap checks if userInfo found in subject
// return true directly if found a match
// subject.kind can only be ServiceAccount, User and Group
func matchSubjectsMap(subject rbacv1.Subject, userInfo authenticationv1.UserInfo, namespace string) bool {
	if strings.Contains(userInfo.Username, saPrefix) {
		return matchServiceAccount(subject, userInfo, namespace)
	}
	return matchUserOrGroup(subject, userInfo)
}

// matchServiceAccount checks if userInfo sa matche the subject sa
// serviceaccount represents as saPrefix:namespace:name in userInfo
func matchServiceAccount(subject rbacv1.Subject, userInfo authenticationv1.UserInfo, namespace string) bool {
	subjectServiceAccount := namespace + ":" + subject.Name
	if userInfo.Username[len(saPrefix):] != subjectServiceAccount {
		return false
	}
	logging.V(3).Info(fmt.Sprintf("found a matched service account not match: %s", subjectServiceAccount))
	return true
}

// matchUserOrGroup checks if userInfo contains user or group info in a subject
func matchUserOrGroup(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	keys := append(userInfo.Groups, userInfo.Username)
	for _, key := range keys {
		if subject.Name == key {
			logging.V(3).Info(fmt.Sprintf("found a matched user/group '%v' in request userInfo: %v", subject.Name, keys))
			return true
		}
	}
	return false
}
