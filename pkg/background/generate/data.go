package generate

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func manageData(log logr.Logger, target kyvernov1.ResourceSpec, data interface{}, synchronize bool, client dclient.Interface) generateResponse {
	if data == nil {
		log.V(4).Info("data is nil - skipping update")
		return newSkipGenerateResponse(nil, target, nil)
	}

	resource, err := datautils.ToMap(data)
	if err != nil {
		return newSkipGenerateResponse(nil, target, err)
	}

	targetObj, err := client.GetResource(context.TODO(), target.GetAPIVersion(), target.GetKind(), target.GetNamespace(), target.GetName())
	if err != nil && apierrors.IsNotFound(err) {
		// the target resource should always exist regardless of synchronize settings
		return newCreateGenerateResponse(resource, target, nil)
	} else if err != nil {
		return newSkipGenerateResponse(nil, target, err)
	}

	log.V(4).Info("found target resource")
	if !synchronize {
		log.V(4).Info("synchronize disabled, skip updating target resource for data")
		return newSkipGenerateResponse(nil, target, nil)
	}

	updateObj := &unstructured.Unstructured{}
	updateObj.SetUnstructuredContent(resource)
	updateObj.SetResourceVersion(targetObj.GetResourceVersion())
	return newUpdateGenerateResponse(updateObj.UnstructuredContent(), target, nil)
}
