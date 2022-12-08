package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type handlers struct {
	client     dclient.Interface
	cpolLister kyvernov2alpha1listers.ClusterCleanupPolicyLister
	polLister  kyvernov2alpha1listers.CleanupPolicyLister
	nsLister   corev1listers.NamespaceLister
}

func New(
	client dclient.Interface,
	cpolLister kyvernov2alpha1listers.ClusterCleanupPolicyLister,
	polLister kyvernov2alpha1listers.CleanupPolicyLister,
	nsLister corev1listers.NamespaceLister,
) *handlers {
	return &handlers{
		client:     client,
		cpolLister: cpolLister,
		polLister:  polLister,
		nsLister:   nsLister,
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

func (h *handlers) lookupPolicy(namespace, name string) (kyvernov2alpha1.CleanupPolicyInterface, error) {
	if namespace == "" {
		return h.cpolLister.Get(name)
	} else {
		return h.polLister.CleanupPolicies(namespace).Get(name)
	}
}

func (h *handlers) executePolicy(ctx context.Context, logger logr.Logger, policy kyvernov2alpha1.CleanupPolicyInterface) error {
	spec := policy.GetSpec()
	kinds := sets.NewString(spec.MatchResources.GetKinds()...)
	var errs []error
	for kind := range kinds {
		logger := logger.WithValues("kind", kind)
		logger.V(5).Info("processing...")
		list, err := h.client.ListResource(ctx, "", kind, policy.GetNamespace(), nil)
		if err != nil {
			logger.Error(err, "failed to list resources")
			errs = append(errs, err)
		} else {
			for i := range list.Items {
				resource := list.Items[i]
				namespace := resource.GetNamespace()
				name := resource.GetName()
				logger := logger.WithValues("name", name, "namespace", namespace)
				if !controllerutils.IsManagedByKyverno(&resource) {
					var nsLabels map[string]string
					if namespace != "" {
						ns, err := h.nsLister.Get(namespace)
						if err != nil {
							logger.Error(err, "failed to get namespace labels")
							errs = append(errs, err)
						}
						nsLabels = ns.GetLabels()
					}
					// match namespaces
					if err := checkNamespace(policy.GetNamespace(), resource); err != nil {
						logger.V(5).Info("resource namespace didn't match policy namespace", "result", err)
					}
					// match resource with match/exclude clause
					matched := checkMatchesResources(resource, spec.MatchResources, nsLabels)
					if matched != nil {
						logger.V(5).Info("resource/match didn't match", "result", matched)
						continue
					}
					if spec.ExcludeResources != nil {
						excluded := checkMatchesResources(resource, *spec.ExcludeResources, nsLabels)
						if excluded == nil {
							logger.V(5).Info("resource/exclude matched")
							continue
						} else {
							logger.V(5).Info("resource/exclude didn't match", "result", excluded)
						}
					}
					logger.V(5).Info("resource matched, it will be deleted...")
					if err := h.client.DeleteResource(ctx, resource.GetAPIVersion(), resource.GetKind(), namespace, name, false); err != nil {
						logger.Error(err, "failed to delete resource")
						errs = append(errs, err)
					}
				}
			}
		}
	}
	return multierr.Combine(errs...)
}
