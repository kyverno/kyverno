package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	kyvernov1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
)

type handlers struct {
	client     dclient.Interface
	cpolLister kyvernov1alpha1listers.ClusterCleanupPolicyLister
	polLister  kyvernov1alpha1listers.CleanupPolicyLister
}

func New(
	client dclient.Interface,
	cpolLister kyvernov1alpha1listers.ClusterCleanupPolicyLister,
	polLister kyvernov1alpha1listers.CleanupPolicyLister,
) *handlers {
	return &handlers{
		client:     client,
		cpolLister: cpolLister,
		polLister:  polLister,
	}
}

func (h *handlers) Cleanup(ctx context.Context, logger logr.Logger, name string, _ time.Time) error {
	logger.Info("cleaning up...")
	namespace, name, err := cache.SplitMetaNamespaceKey(name)
	if err != nil {
		return err
	}
	policy, err := h.lookupPolicy(namespace, name)
	if err != nil {
		return err
	}
	return h.executePolicy(ctx, logger, policy)
}

func (h *handlers) lookupPolicy(namespace, name string) (kyvernov1alpha1.CleanupPolicyInterface, error) {
	if namespace == "" {
		return h.cpolLister.Get(name)
	} else {
		return h.polLister.CleanupPolicies(namespace).Get(name)
	}
}

func (h *handlers) executePolicy(ctx context.Context, logger logr.Logger, policy kyvernov1alpha1.CleanupPolicyInterface) error {
	spec := policy.GetSpec()
	kinds := sets.NewString(spec.MatchResources.GetKinds()...)
	for kind := range kinds {
		logger := logger.WithValues("kind", kind)
		logger.Info("processing...")
		list, err := h.client.ListResource(ctx, "", kind, policy.GetNamespace(), nil)
		if err != nil {
			return err
		}
		for i := range list.Items {
			if !controllerutils.IsManagedByKyverno(&list.Items[i]) {
				logger := logger.WithValues("name", list.Items[i].GetName(), "namespace", list.Items[i].GetNamespace())

				// match resource with match/exclude clause
				matched := checkMatchesResources(list.Items[i], spec.MatchResources, nil, nil)
				if matched != nil {
					logger.Info("resource/match didn't match", "result", matched)
					continue
				}
				excluded := checkMatchesResources(list.Items[i], spec.ExcludeResources, nil, nil)
				if matches != nil {
					logger.Info("resource/exclude didn't match", "result", excluded)
					continue
				}
			}
		}
	}
	return nil
}
