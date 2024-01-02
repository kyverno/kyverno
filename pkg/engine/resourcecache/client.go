package resourcecache

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/resourcecache/cache"
	"k8s.io/client-go/dynamic"
)

type resourceCache struct {
	logger   logr.Logger
	dclient  *dynamic.DynamicClient
	jp       jmespath.Interface
	client   apicall.ClientInterface
	cache    cache.Cache
	apiConf  apicall.APICallConfiguration
	informer v2alpha1.CachedContextEntryInformer
}

func New(logger logr.Logger, dclient *dynamic.DynamicClient, informer v2alpha1.CachedContextEntryInformer, jp jmespath.Interface, client apicall.ClientInterface, config apicall.APICallConfiguration) {

}
