package resourcecache

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

type ResourceCache interface {
	Get(kyvernov1.ContextEntry, enginecontext.Interface) ([]byte, error)
}
