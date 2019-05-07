package interfaces

import (
	policytypes "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type PolicyGetter interface {
	GetPolicies() ([]policytypes.Policy, error)
	GetPolicy(name string) (*policytypes.Policy, error)
	GetCacheInformerSync() cache.InformerSynced
	PatchPolicy(policy string, pt types.PatchType, data []byte) (*policytypes.Policy, error)
	UpdatePolicyViolations(updatedPolicy *policytypes.Policy) error
	LogPolicyError(name, text string)
	LogPolicyInfo(name, text string)
}

type PolicyHandlers interface {
	CreatePolicyHandler(resource interface{})
	UpdatePolicyHandler(oldResource, newResource interface{})
	DeletePolicyHandler(resource interface{})
	GetResourceKey(resource interface{}) string
}
