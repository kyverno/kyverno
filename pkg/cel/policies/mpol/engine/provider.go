package engine

import (
	"context"
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/autogen"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
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
	Fetch(context.Context, bool) []Policy
	MatchesMutateExisting(context.Context, admission.Attributes, *corev1.Namespace) []string
}

func NewKubeProvider(
	ctx context.Context,
	compiler compiler.Compiler,
	mgr ctrl.Manager,
	c openapi.Client,
	polexLister policiesv1beta1listers.PolicyExceptionLister,
	polexEnabled bool,
) (Provider, patch.TypeConverterManager, error) {
	typeConverter := patch.NewTypeConverterManager(nil, c)
	go typeConverter.Run(ctx)

	reconciler := newReconciler(mgr.GetClient(), compiler, polexLister, polexEnabled)
	mpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.MutatingPolicy{})
	nmpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.NamespacedMutatingPolicy{})

	if polexEnabled {
		polexHandler := &handler.Funcs{
			CreateFunc: func(
				ctx context.Context,
				tce event.TypedCreateEvent[client.Object],
				trli workqueue.TypedRateLimitingInterface[reconcile.Request],
			) {
				polex := tce.Object.(*policiesv1beta1.PolicyException)
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
				polex := tce.ObjectNew.(*policiesv1beta1.PolicyException)
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
				polex := tde.Object.(*policiesv1beta1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					trli.Add(reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name: ref.Name,
						},
					})
				}
			},
		}
		mpolBuilder.Watches(&policiesv1beta1.PolicyException{}, polexHandler)
		nmpolBuilder.Watches(&policiesv1beta1.PolicyException{}, polexHandler)
	}
	if err := mpolBuilder.Complete(reconciler); err != nil {
		return nil, typeConverter, fmt.Errorf("failed to construct mutatingpolicy manager: %w", err)
	}
	if err := nmpolBuilder.Complete(reconciler); err != nil {
		return nil, typeConverter, fmt.Errorf("failed to construct mutatingpolicies manager: %w", err)
	}

	return reconciler, typeConverter, nil
}

type staticProvider struct {
	policies []Policy
}

func (p *staticProvider) Fetch(ctx context.Context, mutateExisting bool) []Policy {
	var filtered []Policy
	for _, pol := range p.policies {
		if mutateExisting == pol.Policy.GetSpec().MutateExistingEnabled() {
			filtered = append(filtered, pol)
		}
	}
	return filtered
}

func (r *staticProvider) MatchesMutateExisting(ctx context.Context, attr admission.Attributes, namespace *corev1.Namespace) []string {
	policies := r.Fetch(ctx, true)
	matchedPolicies := []string{}
	for _, mpol := range policies {
		matcher := matching.NewMatcher()
		matchConstraints := mpol.Policy.GetSpec().MatchConstraints
		if ok, err := matcher.Match(&matching.MatchCriteria{Constraints: matchConstraints}, attr, namespace); err != nil || !ok {
			continue
		}

		if mpol.Policy.GetSpec().MatchConditions != nil {
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
	policies []policiesv1beta1.MutatingPolicyLike,
	exceptions []*policiesv1beta1.PolicyException,
) (Provider, error) {
	out := make([]Policy, 0, len(policies))
	for _, policy := range policies {
		var matchedExceptions []*policiesv1beta1.PolicyException
		for _, polex := range exceptions {
			for _, ref := range polex.Spec.PolicyRefs {
				if ref.Name == policy.GetName() && ref.Kind == policy.GetKind() {
					matchedExceptions = append(matchedExceptions, polex)
				}
			}
		}
		compiled, errs := compiler.Compile(policy, matchedExceptions)
		if len(errs) > 0 {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", policy.GetName(), errs.ToAggregate())
		}
		out = append(out, Policy{
			Policy:         policy,
			CompiledPolicy: compiled,
		})
		generated, err := autogen.Autogen(policy)
		if err != nil {
			return nil, err
		}
		for _, gen := range generated {
			// Create a copy of the policy with autogenerated spec
			autogenPolicy := policy.DeepCopyObject().(policiesv1beta1.MutatingPolicyLike)
			*autogenPolicy.GetSpec() = *gen.Spec

			compiled, errs := compiler.Compile(autogenPolicy, matchedExceptions)
			if len(errs) > 0 {
				return nil, fmt.Errorf("failed to compile policy %s (%w)", policy.GetName(), errs.ToAggregate())
			}
			out = append(out, Policy{
				Policy:         autogenPolicy,
				CompiledPolicy: compiled,
			})
		}
	}
	return &staticProvider{policies: out}, nil
}
