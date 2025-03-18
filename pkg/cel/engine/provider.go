package engine

import (
	"context"
	"fmt"
	"sync"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/kyverno/kyverno/pkg/cel/policy"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"golang.org/x/exp/maps"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CompiledValidatingPolicy struct {
	Actions        sets.Set[admissionregistrationv1.ValidationAction]
	Policy         policiesv1alpha1.ValidatingPolicy
	CompiledPolicy policy.CompiledPolicy
}

type CompiledImageVerificationPolicy struct {
	Policy  *policiesv1alpha1.ImageValidatingPolicy
	Actions sets.Set[admissionregistrationv1.ValidationAction]
}

type Provider interface {
	CompiledValidationPolicies(context.Context) ([]CompiledValidatingPolicy, error)
	ImageVerificationPolicies(context.Context) ([]CompiledImageVerificationPolicy, error)
}

type reconcilers struct {
	*policyReconciler
	*ivpolpolicyReconciler
}

type VPolProviderFunc func(context.Context) ([]CompiledValidatingPolicy, error)

func (f VPolProviderFunc) CompiledValidationPolicies(ctx context.Context) ([]CompiledValidatingPolicy, error) {
	return f(ctx)
}

type ImageVerifyPolProviderFunc func(context.Context) ([]CompiledImageVerificationPolicy, error)

func (f ImageVerifyPolProviderFunc) ImageVerificationPolicies(ctx context.Context) ([]CompiledImageVerificationPolicy, error) {
	return f(ctx)
}

func NewProvider(compiler policy.Compiler, vpolicies []policiesv1alpha1.ValidatingPolicy, exceptions []*policiesv1alpha1.CELPolicyException) (VPolProviderFunc, error) {
	compiled := make([]CompiledValidatingPolicy, 0, len(vpolicies))
	for _, vp := range vpolicies {
		var matchedExceptions []policiesv1alpha1.CELPolicyException
		for _, polex := range exceptions {
			for _, ref := range polex.Spec.PolicyRefs {
				if ref.Name == vp.GetName() && ref.Kind == vp.GetKind() {
					matchedExceptions = append(matchedExceptions, *polex)
				}
			}
		}
		policy, err := compiler.CompileValidating(&vp, matchedExceptions)
		if err != nil {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", vp.GetName(), err.ToAggregate())
		}
		actions := sets.New(vp.Spec.ValidationAction...)
		if len(actions) == 0 {
			actions.Insert(admissionregistrationv1.Deny)
		}
		compiled = append(compiled, CompiledValidatingPolicy{
			Actions:        actions,
			Policy:         vp,
			CompiledPolicy: policy,
		})
	}
	provider := func(context.Context) ([]CompiledValidatingPolicy, error) {
		return compiled, nil
	}
	return provider, nil
}

func NewKubeProvider(
	compiler policy.Compiler,
	mgr ctrl.Manager,
	polexLister policiesv1alpha1listers.CELPolicyExceptionLister,
) (Provider, error) {
	exceptionHandlerFuncs := &handler.Funcs{
		CreateFunc: func(
			ctx context.Context,
			tce event.TypedCreateEvent[client.Object],
			trli workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			polex := tce.Object.(*policiesv1alpha1.CELPolicyException)
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
			tue event.TypedUpdateEvent[client.Object],
			trli workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			polex := tue.ObjectNew.(*policiesv1alpha1.CELPolicyException)
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
			polex := tde.Object.(*policiesv1alpha1.CELPolicyException)
			for _, ref := range polex.Spec.PolicyRefs {
				trli.Add(reconcile.Request{
					NamespacedName: client.ObjectKey{
						Name: ref.Name,
					},
				})
			}
		},
	}
	r := newPolicyReconciler(compiler, mgr.GetClient(), polexLister)
	err := ctrl.NewControllerManagedBy(mgr).
		For(&policiesv1alpha1.ValidatingPolicy{}).
		Watches(&policiesv1alpha1.CELPolicyException{}, exceptionHandlerFuncs).
		Complete(r)
	if err != nil {
		return nil, fmt.Errorf("failed to construct manager: %w", err)
	}

	ivpolr := newivPolicyReconciler(mgr.GetClient(), polexLister)
	err = ctrl.NewControllerManagedBy(mgr).
		For(&policiesv1alpha1.ImageValidatingPolicy{}).
		Watches(&policiesv1alpha1.CELPolicyException{}, exceptionHandlerFuncs).
		Complete(ivpolr)
	if err != nil {
		return nil, fmt.Errorf("failed to construct manager: %w", err)
	}

	return reconcilers{r, ivpolr}, nil
}

type policyReconciler struct {
	client      client.Client
	compiler    policy.Compiler
	lock        *sync.RWMutex
	policies    map[string]CompiledValidatingPolicy
	polexLister policiesv1alpha1listers.CELPolicyExceptionLister
}

func newPolicyReconciler(
	compiler policy.Compiler,
	client client.Client,
	polexLister policiesv1alpha1listers.CELPolicyExceptionLister,
) *policyReconciler {
	return &policyReconciler{
		client:      client,
		compiler:    compiler,
		lock:        &sync.RWMutex{},
		policies:    map[string]CompiledValidatingPolicy{},
		polexLister: polexLister,
	}
}

func (r *policyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var policy policiesv1alpha1.ValidatingPolicy
	err := r.client.Get(ctx, req.NamespacedName, &policy)
	if errors.IsNotFound(err) {
		r.lock.Lock()
		defer r.lock.Unlock()
		delete(r.policies, req.NamespacedName.String())
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if policy.GetStatus().Generated {
		r.lock.Lock()
		defer r.lock.Unlock()
		delete(r.policies, req.NamespacedName.String())
		return ctrl.Result{}, nil
	}
	// get exceptions that match the policy
	exceptions, err := listExceptions(r.polexLister, policy.GetName())
	if err != nil {
		return ctrl.Result{}, err
	}
	compiled, errs := r.compiler.CompileValidating(&policy, exceptions)
	if len(errs) > 0 {
		fmt.Println(errs)
		// No need to retry it
		return ctrl.Result{}, nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	actions := sets.New(policy.Spec.ValidationAction...)
	if len(actions) == 0 {
		actions.Insert(admissionregistrationv1.Deny)
	}
	r.policies[req.NamespacedName.String()] = CompiledValidatingPolicy{
		Actions:        actions,
		Policy:         policy,
		CompiledPolicy: compiled,
	}
	return ctrl.Result{}, nil
}

func (r *policyReconciler) CompiledValidationPolicies(ctx context.Context) ([]CompiledValidatingPolicy, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return maps.Values(r.policies), nil
}

func listExceptions(polexLister policiesv1alpha1listers.CELPolicyExceptionLister, policyName string) ([]policiesv1alpha1.CELPolicyException, error) {
	polexList, err := polexLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var exceptions []policiesv1alpha1.CELPolicyException
	for _, polex := range polexList {
		for _, ref := range polex.Spec.PolicyRefs {
			if ref.Name == policyName {
				exceptions = append(exceptions, *polex)
			}
		}
	}
	return exceptions, nil
}

type ivpolpolicyReconciler struct {
	client      client.Client
	lock        *sync.RWMutex
	policies    map[string]CompiledImageVerificationPolicy
	polexLister policiesv1alpha1listers.CELPolicyExceptionLister
}

func newivPolicyReconciler(
	client client.Client,
	polexLister policiesv1alpha1listers.CELPolicyExceptionLister,
) *ivpolpolicyReconciler {
	return &ivpolpolicyReconciler{
		client:      client,
		lock:        &sync.RWMutex{},
		policies:    map[string]CompiledImageVerificationPolicy{},
		polexLister: polexLister,
	}
}

func (r *ivpolpolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var policy policiesv1alpha1.ImageValidatingPolicy
	err := r.client.Get(ctx, req.NamespacedName, &policy)
	if errors.IsNotFound(err) {
		r.lock.Lock()
		defer r.lock.Unlock()
		delete(r.policies, req.NamespacedName.String())
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// todo: exception support
	// get exceptions that match the policy
	// exceptions, err := listExceptions(r.polexLister, policy.GetName())
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	autogeneratedIvPols, err := autogen.GetAutogenRulesImageVerify(&policy)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	actions := sets.New(policy.Spec.ValidationAction...)
	if len(actions) == 0 {
		actions.Insert(admissionregistrationv1.Deny)
	}
	r.policies[req.NamespacedName.String()] = CompiledImageVerificationPolicy{
		Policy:  &policy,
		Actions: actions,
	}
	for _, p := range autogeneratedIvPols {
		namespacedName := types.NamespacedName{
			Name: p.Name,
		}
		r.policies[namespacedName.String()] = CompiledImageVerificationPolicy{
			Policy: &policiesv1alpha1.ImageValidatingPolicy{
				Spec: p.Spec,
			},
			Actions: actions,
		}
	}
	return ctrl.Result{}, nil
}

func (r *ivpolpolicyReconciler) ImageVerificationPolicies(ctx context.Context) ([]CompiledImageVerificationPolicy, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return maps.Values(r.policies), nil
}
