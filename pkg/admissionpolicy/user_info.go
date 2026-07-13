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

// backgroundUsername identifies the reports controller when it evaluates ValidatingAdmissionPolicies
// and MutatingAdmissionPolicies during background scans, where there is no admission user.
const backgroundUsername = "system:serviceaccount:kyverno:kyverno-background-controller"

// NewBackgroundUser returns a user for background evaluation with a non-empty username. The username
// must be non-empty because authenticationv1.UserInfo.Username has the omitempty JSON tag, so an
// empty value is dropped when the admission request is converted to the map the CEL engine reads.
// A missing username makes request.userInfo.username absent and fails any policy that references it.
func NewBackgroundUser() UserInfo {
	return NewUser(authenticationv1.UserInfo{Username: backgroundUsername})
}
