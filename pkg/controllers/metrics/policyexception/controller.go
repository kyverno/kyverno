package policyexception

import (
	"cmp"
	"context"
	"slices"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2"
	policiesv1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	legacyAPIGroup = "kyverno.io"
	celAPIGroup    = "policies.kyverno.io"
)

type info struct {
	apiGroup           string
	exceptionNamespace string
	exceptionName      string
	policyKind         string
	policyName         string
}

type controller struct {
	metrics metrics.PolicyExceptionMetrics

	legacyLister kyvernov2listers.PolicyExceptionLister
	celLister    policiesv1beta1listers.PolicyExceptionLister
	legacySynced cache.InformerSynced
	celSynced    cache.InformerSynced
}

// NewController registers a scrape-time callback backed by the two
// PolicyException informer caches. The callback reports resources from each
// API independently so a temporary failure in one source does not hide the
// other source's observations.
func NewController(
	legacyInformer kyvernov2informers.PolicyExceptionInformer,
	celInformer policiesv1beta1informers.PolicyExceptionInformer,
) {
	c := &controller{
		metrics:      metrics.GetPolicyExceptionMetrics(),
		legacyLister: legacyInformer.Lister(),
		celLister:    celInformer.Lister(),
		legacySynced: legacyInformer.Informer().HasSynced,
		celSynced:    celInformer.Informer().HasSynced,
	}
	if c.metrics == nil {
		return
	}
	if _, err := c.metrics.RegisterCallback(c.report); err != nil {
		logger.Error(err, "failed to register callback for policy exception info metric")
	}
}

func (c *controller) report(ctx context.Context, observer metric.Observer) error {
	if c.legacySynced() {
		exceptions, err := c.legacyLister.List(labels.Everything())
		if err != nil {
			logger.Error(err, "failed to list legacy PolicyExceptions")
		} else {
			recordInfos(ctx, observer, c.metrics, legacyInfos(exceptions))
		}
	} else {
		logger.V(4).Info("legacy PolicyException informer has not synced; skipping metric observations")
	}

	if c.celSynced() {
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

func legacyInfos(exceptions []*kyvernov2.PolicyException) []info {
	infos := make([]info, 0)
	for _, exception := range exceptions {
		base := info{
			apiGroup:           legacyAPIGroup,
			exceptionNamespace: exception.Namespace,
			exceptionName:      exception.Name,
		}
		if len(exception.Spec.Exceptions) == 0 {
			infos = append(infos, base)
			continue
		}

		seen := make(map[string]struct{}, len(exception.Spec.Exceptions))
		for _, reference := range exception.Spec.Exceptions {
			kind := "ClusterPolicy"
			if strings.Contains(reference.PolicyName, "/") {
				kind = "Policy"
			}
			key := kind + "\x00" + reference.PolicyName
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			infos = append(infos, info{
				apiGroup:           base.apiGroup,
				exceptionNamespace: base.exceptionNamespace,
				exceptionName:      base.exceptionName,
				policyKind:         kind,
				policyName:         reference.PolicyName,
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
