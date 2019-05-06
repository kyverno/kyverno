package internalinterfaces

import (
	policytypes "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

// PolicyGetter interface for external API
type PolicyGetter interface {
	GetPolicies() ([]policytypes.Policy, error)
	GetPolicy(name string) (*policytypes.Policy, error)
	GetCacheInformerSync() cache.InformerSynced
	PatchPolicy(policy string, pt types.PatchType, data []byte) (*policytypes.Policy, error)
	Run(stopCh <-chan struct{})
	LogPolicyError(name, text string)
	LogPolicyInfo(name, text string)
}
