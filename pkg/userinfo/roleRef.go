package userinfo

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	client "github.com/nirmata/kyverno/pkg/dclient"
	engine "github.com/nirmata/kyverno/pkg/engine"
	v1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetRoleRef(client *client.Client, request *v1beta1.AdmissionRequest) (roles []string, clusterRoles []string, err error) {
	nsList, err := client.ListResource("Namespace", "", metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get namespace list: %v", err)
	}

	// rolebindings
	for _, ns := range nsList.Items {
		roleBindings, err := client.ListResource("RoleBindings", ns.GetName(), metav1.ListOptions{})
		if err != nil {
			return roles, clusterRoles, fmt.Errorf("failed to list rolebindings: %v", err)
		}

		rs, crs, err := getRoleRefByRoleBindings(roleBindings, request.UserInfo)
		if err != nil {
			return roles, clusterRoles, err
		}
		roles = append(roles, rs...)
		clusterRoles = append(clusterRoles, crs...)
	}

	// clusterrolebindings
	clusterroleBindings, err := client.ListResource("ClusterRoleBindings", "", metav1.ListOptions{})
	if err != nil {
		return roles, clusterRoles, fmt.Errorf("failed to list clusterrolebindings: %v", err)
	}

	crs, err := getRoleRefByClusterRoleBindings(clusterroleBindings, request.UserInfo)
	if err != nil {
		return roles, clusterRoles, err
	}
	clusterRoles = append(clusterRoles, crs...)

	return roles, clusterRoles, nil
}

func getRoleRefByRoleBindings(roleBindings *unstructured.UnstructuredList, userInfo authenticationv1.UserInfo) (roles []string, clusterRoles []string, err error) {
	for _, rolebinding := range roleBindings.Items {
		rb := rolebinding.UnstructuredContent()
		subjects, ok := rb["subjects"]
		if !ok {
			return nil, nil, fmt.Errorf("%s/%s/%s has no subjects field", rolebinding.GetKind(), rolebinding.GetNamespace(), rolebinding.GetName())
		}

		roleRef, ok := rb["roleRef"]
		if !ok {
			return nil, nil, fmt.Errorf("%s/%s/%s has no roleRef", rolebinding.GetKind(), rolebinding.GetNamespace(), rolebinding.GetName())
		}

		for _, subject := range subjects.([]map[string]interface{}) {
			if !matchSubjectsMap(subject, userInfo) {
				continue
			}

			roleRefMap := roleRef.(map[string]interface{})
			switch roleRefMap["kind"] {
			case "role":
				roles = append(roles, roleRefMap["namespace"].(string)+":"+roleRefMap["name"].(string))
			case "clusterRole":
				clusterRoles = append(clusterRoles, roleRefMap["name"].(string))
			}
		}
	}

	return roles, clusterRoles, nil
}

// RoleRef in ClusterRoleBindings can only reference a ClusterRole in the global namespace
func getRoleRefByClusterRoleBindings(clusterroleBindings *unstructured.UnstructuredList, userInfo authenticationv1.UserInfo) (clusterRoles []string, err error) {
	for _, clusterRoleBinding := range clusterroleBindings.Items {
		crb := clusterRoleBinding.UnstructuredContent()
		subjects, ok := crb["subjects"]
		if !ok {
			return nil, fmt.Errorf("%s/%s has no subjects field", clusterRoleBinding.GetKind(), clusterRoleBinding.GetName())
		}

		roleRef, ok := crb["roleRef"]
		if !ok {
			return nil, fmt.Errorf("%s/%s has no roleRef", clusterRoleBinding.GetKind(), clusterRoleBinding.GetName())
		}

		for _, subject := range subjects.([]map[string]interface{}) {
			if !matchSubjectsMap(subject, userInfo) {
				continue
			}

			roleRefMap := roleRef.(map[string]interface{})
			if roleRefMap["kind"] == "clusterRole" {
				clusterRoles = append(clusterRoles, roleRefMap["name"].(string))
			}
		}
	}
	return clusterRoles, nil
}

// matchSubjectsMap checks if userInfo found in subject
// return true directly if found a match
// subject["kind"] can only be ServiceAccount, User and Group
func matchSubjectsMap(subject map[string]interface{}, userInfo authenticationv1.UserInfo) bool {
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
func matchServiceAccount(subject map[string]interface{}, userInfo authenticationv1.UserInfo) bool {
	// checks if subject contains the serviceaccount info
	sa, ok := subject["kind"].(string)
	if !ok {
		glog.V(3).Infof("subject %v has wrong kind field", subject)
		return false
	}

	if sa != "ServiceAccount" {
		glog.V(3).Infof("subject %v has no ServiceAccount info", subject)
		return false
	}

	namespace, ok := subject["namespace"].(string)
	if !ok {
		glog.V(3).Infof("subject %v has wrong namespace field", subject)
		return false
	}

	_ = subject["name"]
	name, ok := subject["name"].(string)
	if !ok {
		glog.V(3).Infof("subject %v has wrong name field", subject)
		return false
	}

	subjectServiceAccount := namespace + ":" + name
	if userInfo.Username[len(engine.SaPrefix):] != subjectServiceAccount {
		glog.V(3).Infof("service account not match, expect %s, got %s", subjectServiceAccount, userInfo.Username[len(engine.SaPrefix):])
		return false
	}

	return true
}

// matchUserOrGroup checks if userInfo contains user or group info in a subject
func matchUserOrGroup(subject map[string]interface{}, userInfo authenticationv1.UserInfo) bool {
	keys := append(userInfo.Groups, userInfo.Username)
	for _, key := range keys {
		if subject["name"].(string) == key {
			return true
		}
	}

	glog.V(3).Infof("user/group '%v' info not found in request userInfo: %v", subject["name"], keys)
	return false
}
