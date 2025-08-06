package user

import "github.com/google/cel-go/common/types"

var ServiceAccountType = types.NewObjectType("user.ServiceAccount")

type ServiceAccount struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}
