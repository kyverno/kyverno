package admissionpolicy

import (
	authenticationv1 "k8s.io/api/authentication/v1"
)

// backgroundUsername provides a stable identity for the background controller during CEL evaluations.
const backgroundUsername = "kyverno:background-evaluation"

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

// NewBackgroundUser returns a sentinal identity to prevent CEL evaluations
//  from crashing on missing keys. 
func NewBackgroundUser() UserInfo {
	return UserInfo{
		userInfo: authenticationv1.UserInfo{
			Username: backgroundUsername,
			UID: "",
			Groups: []string{},
			Extra: map[string]authenticationv1.ExtraValue{},
		},
	}
}