package policyexception

import (
	"cmp"
	"context"
	"slices"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	policiesv1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel/metric"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	legacyAPIGroup = "kyverno.io"
	celAPIGroup    = "policies.kyverno.io"
)

var legacyGVR = schema.GroupVersionResource{Group: legacyAPIGroup, Version: "v2", Resource: "policyexceptions"}

type info struct {
	apiGroup           string
	exceptionNamespace string
	exceptionName      string
	policyKind         string
	policyName         string
}

type controller struct {
	metrics metrics.PolicyExceptionMetrics

	legacyLister cache.GenericLister
	celLister    policiesv1beta1listers.PolicyExceptionLister
	legacySynced cache.InformerSynced
	celSynced    cache.InformerSynced
}

// NewController discovers optional APIs before instantiating their informers.
// Discovery runs only at startup; an unavailable source is skipped until restart.
// Callers start these inventory-only factories without waiting for cache sync.
func NewController(discoveryClient discovery.ServerResourcesInterface, factory kyvernoinformers.SharedInformerFactory, legacyFactory dynamicinformer.DynamicSharedInformerFactory) {
	if metrics.GetPolicyExceptionMetrics() == nil {
		return
	}
	var legacyInformer informers.GenericInformer
	var celInformer policiesv1beta1informers.PolicyExceptionInformer
	if hasPolicyExceptions(discoveryClient, legacyGVR.GroupVersion().String()) {
		legacyInformer = legacyFactory.ForResource(legacyGVR)
	}
	if hasPolicyExceptions(discoveryClient, policiesv1beta1.SchemeGroupVersion.String()) {
		celInformer = factory.Policies().V1beta1().PolicyExceptions()
	}
	newController(legacyInformer, celInformer)
}

func hasPolicyExceptions(client discovery.ServerResourcesInterface, groupVersion string) bool {
	resources, err := client.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to discover PolicyException API; skipping metric source", "groupVersion", groupVersion)
		}
		return false
	}
	for _, resource := range resources.APIResources {
		if resource.Name == "policyexceptions" {
			return true
		}
	}
	return false
}

// newController registers a scrape-time callback backed by the two
// PolicyException informer caches. The callback reports resources from each
// API independently so a temporary failure in one source does not hide the
// other source's observations.
func newController(
	legacyInformer informers.GenericInformer,
	celInformer policiesv1beta1informers.PolicyExceptionInformer,
) {
	c := &controller{metrics: metrics.GetPolicyExceptionMetrics()}
	if c.metrics == nil {
		return
	}
	if legacyInformer != nil {
		c.legacyLister = legacyInformer.Lister()
		c.legacySynced = legacyInformer.Informer().HasSynced
	}
	if celInformer != nil {
		c.celLister = celInformer.Lister()
		c.celSynced = celInformer.Informer().HasSynced
	}
	if _, err := c.metrics.RegisterCallback(c.report); err != nil {
		logger.Error(err, "failed to register callback for policy exception info metric")
	}
}

func (c *controller) report(ctx context.Context, observer metric.Observer) error {
	if c.legacySynced != nil && c.legacySynced() {
		exceptions, err := c.legacyLister.List(labels.Everything())
		if err != nil {
			logger.Error(err, "failed to list legacy PolicyExceptions")
		} else {
			recordInfos(ctx, observer, c.metrics, legacyInfos(exceptions))
		}
	} else {
		logger.V(4).Info("legacy PolicyException informer has not synced; skipping metric observations")
	}

	if c.celSynced != nil && c.celSynced() {
		exceptions, err := c.celLister.List(labels.Everything())
		if err != nil {
			logger.Error(err, "failed to list CEL PolicyExceptions")
		} else {
			recordInfos(ctx, observer, c.metrics, celInfos(exceptions))
		}
	} else {
		logger.V(4).Info("CEL PolicyException informer has not synced; skipping metric observations")
	}

	return nil
}

func recordInfos(ctx context.Context, observer metric.Observer, m metrics.PolicyExceptionMetrics, infos []info) {
	for _, item := range infos {
		m.RecordPolicyExceptionInfo(
			ctx,
			observer,
			item.apiGroup,
			item.exceptionNamespace,
			item.exceptionName,
			item.policyKind,
			item.policyName,
		)
	}
}

func legacyInfos(exceptions []runtime.Object) []info {
	infos := make([]info, 0)
	for _, object := range exceptions {
		exception, ok := object.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		base := info{
			apiGroup:           legacyAPIGroup,
			exceptionNamespace: exception.GetNamespace(),
			exceptionName:      exception.GetName(),
		}
		references, _, _ := unstructured.NestedSlice(exception.Object, "spec", "exceptions")
		if len(references) == 0 {
			infos = append(infos, base)
			continue
		}

		seen := make(map[string]struct{}, len(references))
		for _, reference := range references {
			fields, ok := reference.(map[string]interface{})
			if !ok {
				continue
			}
			policyName, _, _ := unstructured.NestedString(fields, "policyName")
			kind := "ClusterPolicy"
			if strings.Contains(policyName, "/") {
				kind = "Policy"
			}
			key := kind + "\x00" + policyName
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			infos = append(infos, info{
				apiGroup:           base.apiGroup,
				exceptionNamespace: base.exceptionNamespace,
				exceptionName:      base.exceptionName,
				policyKind:         kind,
				policyName:         policyName,
			})
		}
	}
	return sortInfos(infos)
}

func celInfos(exceptions []*policiesv1beta1.PolicyException) []info {
	infos := make([]info, 0)
	for _, exception := range exceptions {
		base := info{
			apiGroup:           celAPIGroup,
			exceptionNamespace: exception.Namespace,
			exceptionName:      exception.Name,
		}
		if len(exception.Spec.PolicyRefs) == 0 {
			infos = append(infos, base)
			continue
		}

		seen := make(map[string]struct{}, len(exception.Spec.PolicyRefs))
		for _, reference := range exception.Spec.PolicyRefs {
			key := reference.Kind + "\x00" + reference.Name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			infos = append(infos, info{
				apiGroup:           base.apiGroup,
				exceptionNamespace: base.exceptionNamespace,
				exceptionName:      base.exceptionName,
				policyKind:         reference.Kind,
				policyName:         reference.Name,
			})
		}
	}
	return sortInfos(infos)
}

func sortInfos(infos []info) []info {
	slices.SortFunc(infos, func(a, b info) int {
		if result := cmp.Compare(a.apiGroup, b.apiGroup); result != 0 {
			return result
		}
		if result := cmp.Compare(a.exceptionNamespace, b.exceptionNamespace); result != 0 {
			return result
		}
		if result := cmp.Compare(a.exceptionName, b.exceptionName); result != 0 {
			return result
		}
		if result := cmp.Compare(a.policyKind, b.policyKind); result != 0 {
			return result
		}
		return cmp.Compare(a.policyName, b.policyName)
	})
	return infos
}
