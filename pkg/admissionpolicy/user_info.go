package admissionpolicy

import (
	authenticationv1 "k8s.io/api/authentication/v1"
)

// UserInfo wraps authenticationv1.UserInfo to implement user.Info interface
type UserInfo struct {
	userInfo authenticationv1.UserInfo
}

func (u UserInfo) GetName() string {
	return u.userInfo.Username
}

func (u UserInfo) GetUID() string {
	return u.userInfo.UID
}

func (u UserInfo) GetGroups() []string {
	return u.userInfo.Groups
}

func (u UserInfo) GetExtra() map[string][]string {
	extra := make(map[string][]string)
	for key, values := range u.userInfo.Extra {
		extra[key] = []string(values)
	}
	return extra
}

func NewUser(userInfo authenticationv1.UserInfo) UserInfo {
	return UserInfo{
		userInfo: userInfo,
	}
}

// backgroundUsername is a sentinel used when a policy is evaluated with no admission user (for
// example background scans of ValidatingAdmissionPolicies and MutatingAdmissionPolicies). It only
// needs to be non-empty: authenticationv1.UserInfo.Username has the omitempty JSON tag, so an empty
// value is dropped when the admission request is converted to the map the CEL engine reads, which
// makes request.userInfo.username absent and fails any policy that references it.
const backgroundUsername = "system:serviceaccount:kyverno:kyverno-background-controller"

// ResolveUser returns a UserInfo for CEL evaluation. It keeps every field of the provided userInfo
// and only fills in the sentinel username when none is set, so request.userInfo.username is always
// present while any supplied username, groups, uid or extra are preserved.
func ResolveUser(userInfo *authenticationv1.UserInfo) UserInfo {
	u := authenticationv1.UserInfo{}
	if userInfo != nil {
		u = *userInfo
	}
	if u.Username == "" {
		u.Username = backgroundUsername
	}
	return NewUser(u)
}
