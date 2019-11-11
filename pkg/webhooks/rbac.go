package webhooks

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

func getRoleRef(client *client.Client, request *v1beta1.AdmissionRequest) (roles []string, clusterRoles []string, err error) {
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

// func newSubjectMap(kind, name, namespace string) map[string]interface{} {
// 	return map[string]interface{}{
// 		"kind":      kind,
// 		"name":      name,
// 		"namespace": namespace,
// 	}
// }

// func filterPolicyByUserInfo(client *client.Client, policies []*v1alpha1.ClusterPolicy, request *v1beta1.AdmissionRequest) ([]*v1alpha1.ClusterPolicy, error) {
// 	var matchesPolicies []*v1alpha1.ClusterPolicy

// 	if request.UserInfo.Username == "" || len(request.UserInfo.Groups) == 0 {
// 		glog.Infof("empty userInfo in the request: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
// 			request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)
// 		return nil, nil
// 	}

// 	for _, p := range policies {
// 		for _, rule := range p.Spec.Rules {
// 			if match := filterByMatchBlock(client, rule.MatchResources, request.UserInfo); match {
// 				matchesPolicies = append(matchesPolicies, p)
// 			}

// 			if exclude := filterByExcludeBlock(client, rule.ExcludeResources, request.UserInfo); !exclude {
// 				matchesPolicies = append(matchesPolicies, p)
// 			}
// 		}
// 	}

// 	return matchesPolicies, nil
// }

// // filterByMatchBlock return true if entire match block is found in userInfo
// func filterByMatchBlock(client *client.Client, match v1alpha1.MatchResources, userInfo authenticationv1.UserInfo) bool {
// 	if reflect.DeepEqual(match, v1alpha1.MatchResources{}) {
// 		return true
// 	}

// 	if !matchSubjects(match.Subjects, userInfo) {
// 		glog.V(3).Infof("Subjects does not match, match subjects: %v, userInfo: %s", match.Subjects, userInfo.String())
// 		return false
// 	}

// 	if !matchRoles(client, match.Roles, userInfo) {
// 		glog.V(3).Infof("Roles does not match, match roles: %v, userInfo: %s", match.Roles, userInfo.String())
// 		return false
// 	}

// 	if !matchClusterRoles(client, match.ClusterRoles, userInfo) {
// 		glog.V(3).Infof("ClusterRoles does not match, match clusterRoles: %v, userInfo: %s", match.ClusterRoles, userInfo.String())
// 		return false
// 	}

// 	return true
// }

// // filterByExcludeBlock return true if entire exclude block found in userInfo
// func filterByExcludeBlock(client *client.Client, exclude v1alpha1.ExcludeResources, userInfo authenticationv1.UserInfo) bool {
// 	if reflect.DeepEqual(exclude, v1alpha1.ExcludeResources{}) {
// 		return false
// 	}

// 	if !matchSubjects(exclude.Subjects, userInfo) {
// 		glog.V(3).Infof("Subjects does not match, exclude subjects: %v, userInfo: %s", exclude.Subjects, userInfo.String())
// 		return false
// 	}

// 	if !matchRoles(client, exclude.Roles, userInfo) {
// 		glog.V(3).Infof("Roles does not match, exclude roles: %v, userInfo: %s", exclude.Roles, userInfo.String())
// 		return false
// 	}

// 	if !matchClusterRoles(client, exclude.ClusterRoles, userInfo) {
// 		glog.V(3).Infof("ClusterRoles does not match, exclude clusterRoles: %v, userInfo: %s", exclude.ClusterRoles, userInfo.String())
// 		return false
// 	}

// 	return true
// }

// // matchSubjects checks if all subjects in the policy match the userInfo in the admission request
// func matchSubjects(subjects []rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
// 	for _, subject := range subjects {
// 		s := newSubjectMap(subject.Kind, subject.Name, subject.Namespace)
// 		if ok := matchSubjectsMap(s, userInfo); !ok {
// 			return false
// 		}
// 	}

// 	// all matches || empty subjects || unknown subject kind
// 	return true
// }

// // matchRoles checks if the given roles matches the roles in this request.UserInfo
// func matchRoles(client *client.Client, roles []string, userInfo authenticationv1.UserInfo) bool {
// 	for _, role := range roles {
// 		// roleInfo = $namespace:name
// 		roleInfo := strings.Split(role, ":")
// 		if len(roleInfo) != 2 {
// 			glog.Errorf("invalid role format, expect namespace:name, found '%s'", role)
// 			return false
// 		}

// 		fieldSelector := fields.Set{
// 			"roleRef.name": roleInfo[1],
// 		}.AsSelector().String()

// 		// rolebindings
// 		roleBindings, err := client.ListResource("RoleBindings", roleInfo[0], metav1.ListOptions{FieldSelector: fieldSelector})
// 		if err != nil {
// 			glog.Errorf("failed to list rolebindings from role '%s'", role)
// 			return false
// 		}

// 		if ok := matchSubjectForRole(roleBindings, userInfo); !ok {
// 			return false
// 		}
// 	}

// 	return true
// }

// func matchClusterRoles(client *client.Client, roles []string, userInfo authenticationv1.UserInfo) bool {
// 	for _, role := range roles {
// 		// roleInfo = $name
// 		fieldSelector := fields.Set{
// 			"roleRef.name": role,
// 		}.AsSelector().String()

// 		nsList, err := client.ListResource("Namespace", "", metav1.ListOptions{})
// 		if err != nil {
// 			glog.Errorf("failed to get namespace list: %v", err)
// 			return false
// 		}

// 		for _, ns := range nsList.Items {
// 			// rolebindings
// 			roleBindings, err := client.ListResource("RoleBindings", ns.GetName(), metav1.ListOptions{FieldSelector: fieldSelector})
// 			if err != nil {
// 				glog.Errorf("failed to list rolebindings from role '%s'", role)
// 				return false
// 			}

// 			if ok := matchSubjectForRole(roleBindings, userInfo); !ok {
// 				return false
// 			}
// 		}

// 		// clusterrolebindings
// 		clusterroleBindings, err := client.ListResource("ClusterRoleBindings", "", metav1.ListOptions{FieldSelector: fieldSelector})
// 		if err != nil {
// 			glog.Errorf("failed to list clusterrolebindings from role '%s'", role)
// 			return false
// 		}

// 		if ok := matchSubjectForRole(clusterroleBindings, userInfo); !ok {
// 			return false
// 		}
// 	}
// 	return true
// }

// func matchSubjectForRole(roleBindings *unstructured.UnstructuredList, userInfo authenticationv1.UserInfo) bool {
// 	subjects, err := getSubjects(roleBindings)
// 	if err != nil {
// 		glog.Errorf("failed to get subjects from rolebindings: %v", err)
// 	}

// 	for _, subject := range subjects {
// 		if ok := matchSubjectsMap(subject, userInfo); !ok {
// 			return false
// 		}
// 	}
// 	return true
// }

// func getSubjects(bindings *unstructured.UnstructuredList) ([]map[string]interface{}, error) {
// 	var subjectsLists []map[string]interface{}
// 	for _, binding := range bindings.Items {
// 		bindingMap := binding.UnstructuredContent()
// 		subjects, ok := bindingMap["subjects"]
// 		if !ok {
// 			return nil, fmt.Errorf("missing subjects in %s/%s", binding.GetKind(), binding.GetName())
// 		}

// 		subjectsList, ok := subjects.([]map[string]interface{})
// 		if !ok {
// 			return nil, fmt.Errorf("wrong type of subjects in %s/%s, expect: %T, found: %T",
// 				binding.GetKind(), binding.GetName(), subjectsList, subjects)
// 		}
// 		subjectsLists = append(subjectsLists, subjectsList...)
// 	}

// 	return subjectsLists, nil
// }
