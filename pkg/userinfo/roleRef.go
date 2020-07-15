package userinfo

import (
	"fmt"
	"strings"

	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	rbaclister "k8s.io/client-go/listers/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	clusterrolekind = "ClusterRole"
	rolekind        = "Role"
	SaPrefix        = "system:serviceaccount:"
)

var defaultSuffixs = []string{"system:", "kyverno:"}

//GetRoleRef gets the list of roles and cluster roles for the incoming api-request
func GetRoleRef(rbLister rbaclister.RoleBindingLister, crbLister rbaclister.ClusterRoleBindingLister, request *v1beta1.AdmissionRequest) (roles []string, clusterRoles []string, err error) {
	keys := append(request.UserInfo.Groups, request.UserInfo.Username)
	if utils.SliceContains(keys, engine.ExcludeUserInfo...) {
		return
	}

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

			switch rolebinding.RoleRef.Kind {
			case rolekind:
				roles = append(roles, rolebinding.Namespace+":"+rolebinding.RoleRef.Name)
			case clusterrolekind:
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

			if clusterRoleBinding.RoleRef.Kind == clusterrolekind {
				clusterRoles = append(clusterRoles, clusterRoleBinding.RoleRef.Name)
			}
		}
	}
	return clusterRoles, nil
}

// matchSubjectsMap checks if userInfo found in subject
// return true directly if found a match
// subject.kind can only be ServiceAccount, User and Group
func matchSubjectsMap(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	// ServiceAccount
	if strings.Contains(userInfo.Username, SaPrefix) {
		return matchServiceAccount(subject, userInfo)
	} else {
		// User or Group
		return matchUserOrGroup(subject, userInfo)
	}
}

// matchServiceAccount checks if userInfo sa matche the subject sa
// serviceaccount represents as saPrefix:namespace:name in userInfo
func matchServiceAccount(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	subjectServiceAccount := subject.Namespace + ":" + subject.Name
	if userInfo.Username[len(SaPrefix):] != subjectServiceAccount {
		return false
	}

	log.Log.V(3).Info(fmt.Sprintf("found a matched service account not match: %s", subjectServiceAccount))
	return true
}

// matchUserOrGroup checks if userInfo contains user or group info in a subject
func matchUserOrGroup(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	keys := append(userInfo.Groups, userInfo.Username)
	for _, key := range keys {
		if subject.Name == key {
			log.Log.V(3).Info(fmt.Sprintf("found a matched user/group '%v' in request userInfo: %v", subject.Name, keys))
			return true
		}
	}

	return false
}

//IsRoleAuthorize is role authorize or not
func IsRoleAuthorize(rbLister rbaclister.RoleBindingLister, crbLister rbaclister.ClusterRoleBindingLister, rLister rbaclister.RoleLister, crLister rbaclister.ClusterRoleLister, request *v1beta1.AdmissionRequest) (bool, error) {
	if strings.Contains(request.UserInfo.Username, SaPrefix) {
		roles, clusterRoles, err := GetRoleRef(rbLister, crbLister, request)
		if err != nil {
			return false, err
		}

		for _, e := range clusterRoles {
			if strings.Contains(e, "kyverno:") {
				return true, nil
			}
			role, err := crLister.Get(e)
			if err != nil {
				return false, err
			}
			labels := role.GetLabels()

			if labels["kubernetes.io/bootstrapping"] == "rbac-defaults" {
				return true, nil
			}
		}
		for _, e := range roles {
			roleData := strings.Split(e, ":")
			role, err := rLister.Roles(roleData[0]).Get(roleData[1])
			if err != nil {
				return false, err
			}
			labels := role.GetLabels()
			if !strings.Contains(e, "kyverno:") {
				if labels["kubernetes.io/bootstrapping"] == "rbac-defaults" {
					return true, nil
				}
			}
		}
		return true, nil
	}
	// User or Group
	excludeDevelopmentRole := []string{"minikube-user", "kubernetes-admin"}
	for _, e := range excludeDevelopmentRole {
		if strings.Contains(request.UserInfo.Username, e) {
			return false, nil
		}
	}
	var matchedRoles []bool
	for _, e := range request.UserInfo.Groups {
		for _, defaultSuffix := range defaultSuffixs {
			if strings.Contains(e, defaultSuffix) {
				matchedRoles = append(matchedRoles, true)
				break
			}
		}
	}
	if len(matchedRoles) == len(request.UserInfo.Groups) {
		return true, nil
	}

	return false, nil
}
