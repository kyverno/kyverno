package generate

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func manageClone(log logr.Logger, target, sourceSpec kyvernov1.ResourceSpec, policy kyvernov1.PolicyInterface, ur kyvernov1beta1.UpdateRequest, rule kyvernov1.Rule, client dclient.Interface) generateResponse {
	source := sourceSpec
	clone := rule.Generation
	if clone.Clone.Name != "" {
		source = kyvernov1.ResourceSpec{
			APIVersion: target.GetAPIVersion(),
			Kind:       target.GetKind(),
			Namespace:  clone.Clone.Namespace,
			Name:       clone.Clone.Name,
		}
	}

	if source.GetNamespace() == "" {
		log.V(4).Info("namespace is optional in case of cluster scope resource", "source namespace", source.GetNamespace())
	}

	if source.GetNamespace() == target.GetNamespace() && source.GetName() == target.GetName() ||
		(rule.Generation.CloneList.Kinds != nil) && (source.GetNamespace() == target.GetNamespace()) {
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
	if err != nil {
		if apierrors.IsNotFound(err) && len(ur.Status.GeneratedResources) != 0 && !clone.Synchronize {
			log.V(4).Info("synchronization is disabled, recreation will be skipped", "target resource", targetObj)
			return newSkipGenerateResponse(nil, target, nil)
		}
		if apierrors.IsNotFound(err) {
			return newCreateGenerateResponse(sourceObjCopy.UnstructuredContent(), target, nil)
		}
		return newSkipGenerateResponse(nil, target, fmt.Errorf("failed to get the target source: %v", err))
	}

	if targetObj != nil {
		if !policy.GetSpec().UseServerSideApply {
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

func manageCloneList(log logr.Logger, targetNamespace string, ur kyvernov1beta1.UpdateRequest, policy kyvernov1.PolicyInterface, rule kyvernov1.Rule, client dclient.Interface) []generateResponse {
	var responses []generateResponse
	cloneList := rule.Generation.CloneList
	sourceNamespace := cloneList.Namespace
	kinds := cloneList.Kinds
	for _, kind := range kinds {
		apiVersion, kind := kubeutils.GetKindFromGVK(kind)
		sources, err := client.ListResource(context.TODO(), apiVersion, kind, sourceNamespace, cloneList.Selector)
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
			responses = append(responses,
				manageClone(log, target, newResourceSpec(source.GetAPIVersion(), source.GetKind(), source.GetNamespace(), source.GetName()), policy, ur, rule, client))
		}
	}
	return responses
}
