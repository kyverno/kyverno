package match

import (
	"golang.org/x/exp/slices"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// CheckSubjects return true if one of ruleSubjects exist in userInfo
func CheckSubjects(
	ruleSubjects []rbacv1.Subject,
	userInfo authenticationv1.UserInfo,
	excludeGroupRole []string,
) bool {
	const SaPrefix = "system:serviceaccount:"
	userGroups := append(userInfo.Groups, userInfo.Username)
	// TODO: see issue https://github.com/kyverno/kyverno/issues/861
	for _, e := range excludeGroupRole {
		ruleSubjects = append(ruleSubjects, rbacv1.Subject{Kind: "Group", Name: e})
	}
	for _, subject := range ruleSubjects {
		switch subject.Kind {
		case "ServiceAccount":
			if len(userInfo.Username) <= len(SaPrefix) {
				continue
			}
			subjectServiceAccount := subject.Namespace + ":" + subject.Name
			if userInfo.Username[len(SaPrefix):] == subjectServiceAccount {
				return true
			}
		case "User", "Group":
			if slices.Contains(userGroups, subject.Name) {
				return true
			}
		}
	}
	return false
}
