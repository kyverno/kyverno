package match

import (
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// CheckSubjects return true if one of ruleSubjects exist in userInfo
func CheckSubjects(ruleSubjects []rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	for _, subject := range ruleSubjects {
		switch subject.Kind {
		case rbacv1.ServiceAccountKind:
			username := "system:serviceaccount:" + subject.Namespace + ":" + subject.Name
			if wildcard.Match(username, userInfo.Username) {
				return true
			}
		case rbacv1.GroupKind:
			for _, group := range userInfo.Groups {
				if wildcard.Match(subject.Name, group) {
					return true
				}
			}
		case rbacv1.UserKind:
			if wildcard.Match(subject.Name, userInfo.Username) {
				return true
			}
		}
	}
	return false
}
