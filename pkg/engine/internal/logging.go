package internal

import (
	"reflect"

	"github.com/go-logr/logr"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func BuildLogger(ctx engineapi.PolicyContext) logr.Logger {
	logger := logging.WithName("EngineValidate").WithValues("policy", ctx.Policy().GetName())
	newResource := ctx.NewResource()
	oldResource := ctx.OldResource()
	if reflect.DeepEqual(newResource, unstructured.Unstructured{}) {
		logger = logger.WithValues("kind", oldResource.GetKind(), "namespace", oldResource.GetNamespace(), "name", oldResource.GetName())
	} else {
		logger = logger.WithValues("kind", newResource.GetKind(), "namespace", newResource.GetNamespace(), "name", newResource.GetName())
	}
	return logger
}
