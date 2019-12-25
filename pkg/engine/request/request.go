package request

import (
	authenticationv1 "k8s.io/api/authentication/v1"
)

// RequestInfo contains permission info carried in an admission request
type RequestInfo struct {
	// Roles is a list of possible role send the request
	Roles []string `json:"roles"`
	// ClusterRoles is a list of possible clusterRoles send the request
	ClusterRoles []string `json:"clusterRoles"`
	// UserInfo is the userInfo carried in the admission request
	AdmissionUserInfo authenticationv1.UserInfo `json:"userInfo"`
}
