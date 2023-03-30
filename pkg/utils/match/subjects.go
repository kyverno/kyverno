package match

import (
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// CheckSubjects return true if one of ruleSubjects exist in userInfo
func CheckSubjects(
	ruleSubjects []rbacv1.Subject,
	userInfo authenticationv1.UserInfo,
) bool {
	for _, subject := range ruleSubjects {
		switch subject.Kind {
		case rbacv1.ServiceAccountKind:
			username := "system:serviceaccount:" + subject.Namespace + ":" + subject.Name
			if userInfo.Username == username {
				return true
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
