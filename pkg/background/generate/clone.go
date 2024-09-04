package generate

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func manageClone(log logr.Logger, target, sourceSpec kyvernov1.ResourceSpec, severSideApply bool, pattern kyvernov1.GeneratePattern, client dclient.Interface) generateResponse {
	source := sourceSpec
	if pattern.Clone.Name != "" {
		source = kyvernov1.ResourceSpec{
			APIVersion: target.GetAPIVersion(),
			Kind:       target.GetKind(),
			Namespace:  pattern.Clone.Namespace,
			Name:       pattern.Clone.Name,
		}
	}

	if source.GetNamespace() == "" {
		log.V(4).Info("namespace is optional in case of cluster scope resource", "source namespace", source.GetNamespace())
	}

	if source.GetNamespace() == target.GetNamespace() && source.GetName() == target.GetName() {
		log.V(4).Info("skip resource self-clone")
		return newSkipGenerateResponse(nil, target, nil)
	}

	sourceObj, err := client.GetResource(context.TODO(), source.GetAPIVersion(), source.GetKind(), source.GetNamespace(), source.GetName())
	if err != nil {
		return newSkipGenerateResponse(nil, target, fmt.Errorf("source resource %s not found: %v", target.String(), err))
	}

	if err := updateSourceLabel(client, sourceObj); err != nil {
		log.Error(err, "failed to add labels to the source", "kind", sourceObj.GetKind(), "namespace", sourceObj.GetNamespace(), "name", sourceObj.GetName())
	}

	sourceObjCopy := sourceObj.DeepCopy()
	addSourceLabels(sourceObjCopy)

	if sourceObjCopy.GetNamespace() != target.GetNamespace() && sourceObjCopy.GetOwnerReferences() != nil {
		sourceObjCopy.SetOwnerReferences(nil)
	}
	// Clean up parameters that shouldn't be copied
	sourceObjCopy.SetUID("")
	sourceObjCopy.SetSelfLink("")
	var emptyTime metav1.Time
	sourceObjCopy.SetCreationTimestamp(emptyTime)
	sourceObjCopy.SetManagedFields(nil)
	sourceObjCopy.SetResourceVersion("")

	targetObj, err := client.GetResource(context.TODO(), target.GetAPIVersion(), target.GetKind(), target.GetNamespace(), target.GetName())
	if err != nil && apierrors.IsNotFound(err) {
		// the target resource should always exist regardless of synchronize settings
		return newCreateGenerateResponse(sourceObjCopy.UnstructuredContent(), target, nil)
	}

	if targetObj != nil {
		if !severSideApply {
			sourceObjCopy.SetUID(targetObj.GetUID())
			sourceObjCopy.SetSelfLink(targetObj.GetSelfLink())
			sourceObjCopy.SetCreationTimestamp(targetObj.GetCreationTimestamp())
			sourceObjCopy.SetManagedFields(targetObj.GetManagedFields())
			sourceObjCopy.SetResourceVersion(targetObj.GetResourceVersion())
			if datautils.DeepEqual(sourceObjCopy, targetObj) {
				return newSkipGenerateResponse(nil, target, nil)
			}
		}
		return newUpdateGenerateResponse(sourceObjCopy.UnstructuredContent(), target, nil)
	}

	return newCreateGenerateResponse(sourceObjCopy.UnstructuredContent(), target, nil)
}

func manageCloneList(log logr.Logger, targetNamespace string, severSideApply bool, pattern kyvernov1.GeneratePattern, client dclient.Interface) []generateResponse {
	var responses []generateResponse
	sourceNamespace := pattern.CloneList.Namespace
	kinds := pattern.CloneList.Kinds
	for _, kind := range kinds {
		apiVersion, kind := kubeutils.GetKindFromGVK(kind)
		sources, err := client.ListResource(context.TODO(), apiVersion, kind, sourceNamespace, pattern.CloneList.Selector)
		if err != nil {
			responses = append(responses,
				newSkipGenerateResponse(
					nil,
					newResourceSpec(apiVersion, kind, targetNamespace, ""),
					fmt.Errorf("failed to list source resource for cloneList %s %s/%s. %v", apiVersion, kind, sourceNamespace, err),
				),
			)
		}

		for _, source := range sources.Items {
			target := newResourceSpec(source.GetAPIVersion(), source.GetKind(), targetNamespace, source.GetName())

			if (pattern.CloneList.Kinds != nil) && (source.GetNamespace() == target.GetNamespace()) {
				log.V(4).Info("skip resource self-clone")
				responses = append(responses, newSkipGenerateResponse(nil, target, nil))
				continue
			}
			responses = append(responses,
				manageClone(log, target, newResourceSpec(source.GetAPIVersion(), source.GetKind(), source.GetNamespace(), source.GetName()), severSideApply, pattern, client))
		}
	}
	return responses
}
