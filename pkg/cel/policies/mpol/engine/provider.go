package engine

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/autogen"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/client-go/openapi"
	workqueue "k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	event "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Provider interface {
	Fetch(context.Context, bool) ([]Policy, error)
	MatchesMutateExisting(context.Context, admission.Attributes, *corev1.Namespace) []string
}

func NewKubeProvider(
	ctx context.Context,
	compiler compiler.Compiler,
	mgr ctrl.Manager,
	c openapi.Client,
	polexLister policiesv1alpha1listers.PolicyExceptionLister,
	polexEnabled bool,
) (Provider, patch.TypeConverterManager, error) {
	typeConverter := patch.NewTypeConverterManager(nil, c)
	go typeConverter.Run(ctx)

	reconciler := newReconciler(mgr.GetClient(), compiler, polexLister, polexEnabled)
	builder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1alpha1.MutatingPolicy{})
	if polexEnabled {
		polexHandler := &handler.Funcs{
			CreateFunc: func(
				ctx context.Context,
				tce event.TypedCreateEvent[client.Object],
				trli workqueue.TypedRateLimitingInterface[reconcile.Request],
			) {
				polex := tce.Object.(*policiesv1alpha1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					trli.Add(reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name: ref.Name,
						},
					})
				}
			},
			UpdateFunc: func(
				ctx context.Context,
				tce event.TypedUpdateEvent[client.Object],
				trli workqueue.TypedRateLimitingInterface[reconcile.Request],
			) {
				polex := tce.ObjectNew.(*policiesv1alpha1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					trli.Add(reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name: ref.Name,
						},
					})
				}
			},
			DeleteFunc: func(
				ctx context.Context,
				tde event.TypedDeleteEvent[client.Object],
				trli workqueue.TypedRateLimitingInterface[reconcile.Request],
			) {
				polex := tde.Object.(*policiesv1alpha1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					trli.Add(reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name: ref.Name,
						},
					})
				}
			},
		}
		builder.Watches(&policiesv1alpha1.PolicyException{}, polexHandler)
	}
	if err := builder.Complete(reconciler); err != nil {
		return nil, typeConverter, fmt.Errorf("failed to construct mutatingpolicies manager: %w", err)
	}

	return reconciler, typeConverter, nil
}

type staticProvider struct {
	policies []Policy
}

func (p *staticProvider) Fetch(ctx context.Context, mutateExisting bool) ([]Policy, error) {
	var filtered []Policy
	for _, pol := range p.policies {
		if mutateExisting == pol.Policy.GetSpec().MutateExistingEnabled() {
			filtered = append(filtered, pol)
		}
	}
	return filtered, nil
}

func (r *staticProvider) MatchesMutateExisting(ctx context.Context, attr admission.Attributes, namespace *corev1.Namespace) []string {
	policies, err := r.Fetch(ctx, true)
	if err != nil {
		return nil
	}

	matchedPolicies := []string{}
	for _, mpol := range policies {
		matcher := matching.NewMatcher()
		matchConstraints := mpol.Policy.GetMatchConstraints()
		if ok, err := matcher.Match(&matching.MatchCriteria{Constraints: &matchConstraints}, attr, namespace); err != nil || !ok {
			continue
		}

		if mpol.Policy.GetSpec().GetMatchConditions() != nil {
			if !mpol.CompiledPolicy.MatchesConditions(ctx, attr, namespace) {
				continue
			}
		}
		matchedPolicies = append(matchedPolicies, mpol.Policy.GetName())
	}
	return matchedPolicies
}

func NewProvider(
	compiler compiler.Compiler,
	policies []v1alpha1.MutatingPolicy,
	exceptions []*v1alpha1.PolicyException,
) (Provider, error) {
	out := make([]Policy, 0, len(policies))
	for _, policy := range policies {
		var matchedExceptions []*v1alpha1.PolicyException
		for _, polex := range exceptions {
			for _, ref := range polex.Spec.PolicyRefs {
				if ref.Name == policy.GetName() && ref.Kind == policy.GetKind() {
					matchedExceptions = append(matchedExceptions, polex)
				}
			}
		}
		compiled, errs := compiler.Compile(&policy, matchedExceptions)
		if len(errs) > 0 {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", policy.GetName(), errs.ToAggregate())
		}
		out = append(out, Policy{
			Policy:         policy,
			CompiledPolicy: compiled,
		})
		generated, err := autogen.Autogen(&policy)
		if err != nil {
			return nil, err
		}
		for _, autogen := range generated {
			policy.Spec = *autogen.Spec
			compiled, errs := compiler.Compile(&policy, matchedExceptions)
			if len(errs) > 0 {
				return nil, fmt.Errorf("failed to compile policy %s (%w)", policy.GetName(), errs.ToAggregate())
			}
			out = append(out, Policy{
				Policy:         policy,
				CompiledPolicy: compiled,
			})
		}
	}
	return &staticProvider{policies: out}, nil
}
