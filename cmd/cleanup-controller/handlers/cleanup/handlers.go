package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/event"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"github.com/kyverno/kyverno/pkg/utils/match"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

type handlers struct {
	client     dclient.Interface
	cpolLister kyvernov2alpha1listers.ClusterCleanupPolicyLister
	polLister  kyvernov2alpha1listers.CleanupPolicyLister
	nsLister   corev1listers.NamespaceLister
	recorder   record.EventRecorder
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
		recorder:   newRecorder(client),
	}
}

func (h *handlers) Cleanup(ctx context.Context, logger logr.Logger, name string, _ time.Time) error {
	logger.Info("cleaning up...")
	defer logger.Info("done")
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
	kinds := sets.New(spec.MatchResources.GetKinds()...)
	debug := logger.V(4)
	var errs []error
	for kind := range kinds {
		debug := debug.WithValues("kind", kind)
		debug.Info("processing...")
		list, err := h.client.ListResource(ctx, "", kind, policy.GetNamespace(), nil)
		if err != nil {
			debug.Error(err, "failed to list resources")
			errs = append(errs, err)
		} else {
			for i := range list.Items {
				resource := list.Items[i]
				namespace := resource.GetNamespace()
				name := resource.GetName()
				debug := debug.WithValues("name", name, "namespace", namespace)
				if !controllerutils.IsManagedByKyverno(&resource) {
					var nsLabels map[string]string
					if namespace != "" {
						ns, err := h.nsLister.Get(namespace)
						if err != nil {
							debug.Error(err, "failed to get namespace labels")
							errs = append(errs, err)
						}
						nsLabels = ns.GetLabels()
					}
					// match namespaces
					if err := match.CheckNamespace(policy.GetNamespace(), resource); err != nil {
						debug.Info("resource namespace didn't match policy namespace", "result", err)
					}
					// match resource with match/exclude clause
					matched := match.CheckMatchesResources(
						resource,
						spec.MatchResources,
						nsLabels,
						nil,
						"",
						// TODO(eddycharly): we don't have user info here, we should check that
						// we don't have user conditions in the policy rule
						kyvernov1beta1.RequestInfo{},
						nil,
					)
					if matched != nil {
						debug.Info("resource/match didn't match", "result", matched)
						continue
					}
					if spec.ExcludeResources != nil {
						excluded := match.CheckMatchesResources(
							resource,
							*spec.ExcludeResources,
							nsLabels,
							nil,
							"",
							// TODO(eddycharly): we don't have user info here, we should check that
							// we don't have user conditions in the policy rule
							kyvernov1beta1.RequestInfo{},
							nil,
						)
						if excluded == nil {
							debug.Info("resource/exclude matched")
							continue
						} else {
							debug.Info("resource/exclude didn't match", "result", excluded)
						}
					}
					// check conditions
					if spec.Conditions != nil {
						enginectx := enginecontext.NewContext()
						if err := enginectx.AddTargetResource(resource.Object); err != nil {
							debug.Error(err, "failed to add resource in context")
							errs = append(errs, err)
							continue
						}
						if err := enginectx.AddNamespace(resource.GetNamespace()); err != nil {
							debug.Error(err, "failed to add namespace in context")
							errs = append(errs, err)
							continue
						}
						if err := enginectx.AddImageInfos(&resource); err != nil {
							debug.Error(err, "failed to add image infos in context")
							errs = append(errs, err)
							continue
						}
						passed, err := checkAnyAllConditions(logger, enginectx, *spec.Conditions)
						if err != nil {
							debug.Error(err, "failed to check condition")
							errs = append(errs, err)
							continue
						}
						if !passed {
							debug.Info("conditions did not pass")
							continue
						}
					}
					logger.WithValues("name", name, "namespace", namespace).Info("resource matched, it will be deleted...")
					if err := h.client.DeleteResource(ctx, resource.GetAPIVersion(), resource.GetKind(), namespace, name, false); err != nil {
						debug.Error(err, "failed to delete resource")
						errs = append(errs, err)
						h.createEvent(policy, resource, err)
					} else {
						debug.Info("deleted")
						h.createEvent(policy, resource, nil)
					}
				}
			}
		}
	}
	return multierr.Combine(errs...)
}

func (h *handlers) createEvent(policy kyvernov2alpha1.CleanupPolicyInterface, resource unstructured.Unstructured, err error) {
	var cleanuppol runtime.Object
	if policy.GetNamespace() == "" {
		cleanuppol = policy.(*kyvernov2alpha1.ClusterCleanupPolicy)
	} else if policy.GetNamespace() != "" {
		cleanuppol = policy.(*kyvernov2alpha1.CleanupPolicy)
	}

	switch err == nil {
	case true:
		msg := fmt.Sprintf("successfully cleaned up the target resource %v/%v/%v", resource.GetKind(), resource.GetNamespace(), resource.GetName())
		h.recorder.Event(cleanuppol, corev1.EventTypeNormal, event.PolicyApplied.String(), msg)
	case false:
		msg := fmt.Sprintf("failed to clean up the target resource %v/%v/%v: %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err.Error())
		h.recorder.Event(cleanuppol, corev1.EventTypeWarning, event.PolicyError.String(), msg)
	}
}
