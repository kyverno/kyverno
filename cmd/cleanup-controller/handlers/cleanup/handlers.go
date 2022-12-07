package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	kyvernov1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"go.uber.org/multierr"
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
	var errs []error
	for kind := range kinds {
		logger := logger.WithValues("kind", kind)
		logger.Info("processing...")
		list, err := h.client.ListResource(ctx, "", kind, policy.GetNamespace(), nil)
		if err != nil {
			logger.Error(err, "failed to list resources")
			errs = append(errs, err)
		} else {
			for i := range list.Items {
				resource := list.Items[i]
				if !controllerutils.IsManagedByKyverno(&resource) {
					logger := logger.WithValues("name", resource.GetName(), "namespace", resource.GetNamespace())
					// match namespaces
					if err := checkNamespace(policy.GetNamespace(), resource); err != nil {
						logger.Info("resource namespace didn't match policy namespace", "result", err)
					}
					// match resource with match/exclude clause
					matched := checkMatchesResources(resource, spec.MatchResources, nil, nil)
					if matched != nil {
						logger.Info("resource/match didn't match", "result", matched)
						continue
					}
					if spec.ExcludeResources != nil {
						excluded := checkMatchesResources(resource, *spec.ExcludeResources, nil, nil)
						if excluded == nil {
							logger.Info("resource/exclude matched")
							continue
						} else {
							logger.Info("resource/exclude didn't match", "result", excluded)
						}
					}
					logger.Info("resource matched, it will be deleted...")
					if err := h.client.DeleteResource(
						ctx,
						resource.GetAPIVersion(),
						resource.GetKind(),
						resource.GetNamespace(),
						resource.GetName(),
						false,
					); err != nil {
						logger.Error(err, "failed to delete resource")
						errs = append(errs, err)
					}
				}
			}
		}
	}
	return multierr.Combine(errs...)
}
