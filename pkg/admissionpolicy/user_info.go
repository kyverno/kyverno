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

// backgroundUsername is a sentinel identity used when a policy is evaluated with no admission user,
// such as a background scan of a ValidatingAdmissionPolicy or MutatingAdmissionPolicy. UserInfo
// fields are omitempty, so an empty username, uid or groups is dropped when the request is converted
// to the map the CEL engine reads, making request.userInfo.<field> absent and failing any policy
// that references it. The value only needs to be non-empty; it is not a valid serviceaccount identity
// (no system:serviceaccount: prefix) and not a privileged group, so it does not match policies that
// allowlist or deny real service accounts or groups such as system:masters.
const backgroundUsername = "system:kyverno:background-scan"

// ResolveUser returns a UserInfo for CEL evaluation. It preserves every provided field and fills the
// sentinel identity into username, uid and groups only when they are empty, so those keys are present
// during background scans while any supplied value is kept. Extra is left as-is: it is keyed by
// arbitrary strings, so a sentinel entry could not satisfy request.userInfo.extra["<key>"] accesses
// and would defeat has() guards that policies use to make such accesses safe.
func ResolveUser(userInfo *authenticationv1.UserInfo) UserInfo {
	u := authenticationv1.UserInfo{}
	if userInfo != nil {
		u = *userInfo
	}
	if u.Username == "" {
		u.Username = backgroundUsername
	}
	if u.UID == "" {
		u.UID = backgroundUsername
	}
	if len(u.Groups) == 0 {
		u.Groups = []string{backgroundUsername}
	}
	return NewUser(u)
}
