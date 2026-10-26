package engine

import (
	"context"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PolicyExceptionLister interface {
	List(labels.Selector) ([]*policiesv1beta1.PolicyException, error)
}

func NewPolicyExceptionLister(lister policiesv1beta1listers.PolicyExceptionLister, namespace string) PolicyExceptionLister {
	if namespace == "" || namespace == "*" {
		return lister
	}

	return lister.PolicyExceptions(namespace)
}

// managerPolicyExceptionLister implements PolicyExceptionLister on top of a
// controller-runtime cache reader (typically a ctrl.Manager's client) instead of a
// standalone client-go SharedInformerFactory.
//
// The CEL policy engines (vpol, ivpol, mpol) register their PolicyException watch on
// the controller-runtime manager cache and cache the compiled result of a reconcile
// until the next triggering event. If exceptions were read from a *different* watch
// stream than the one that triggered the reconcile, the two caches could observe the
// same object at different times: a reconcile could run before the second cache had
// caught up, compile the policy without the exception, and never be retried, leaving
// the exception permanently ignored on that replica. Sourcing the list from the same
// manager cache that delivers the watch event guarantees the object is already present
// by the time a reconcile it triggered runs, since controller-runtime only invokes
// watch handlers after updating the cache's local store.
type managerPolicyExceptionLister struct {
	reader    client.Reader
	namespace string
}

// NewManagerPolicyExceptionLister returns a PolicyExceptionLister backed by reader,
// scoped to namespace (all namespaces if empty or "*"). Pass the same ctrl.Manager
// client used to register the PolicyException watch that triggers reconciliation, so
// reads and watch events are served from a single cache.
func NewManagerPolicyExceptionLister(reader client.Reader, namespace string) PolicyExceptionLister {
	return &managerPolicyExceptionLister{reader: reader, namespace: namespace}
}

func (l *managerPolicyExceptionLister) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	var list policiesv1beta1.PolicyExceptionList
	opts := []client.ListOption{client.MatchingLabelsSelector{Selector: selector}}
	if l.namespace != "" && l.namespace != "*" {
		opts = append(opts, client.InNamespace(l.namespace))
	}
	if err := l.reader.List(context.Background(), &list, opts...); err != nil {
		return nil, err
	}
	out := make([]*policiesv1beta1.PolicyException, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, &list.Items[i])
	}
	return out, nil
}

func ListExceptions(lister PolicyExceptionLister, kind, name string) ([]*policiesv1beta1.PolicyException, error) {
	exceptions, err := lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return MatchExceptions(exceptions, kind, name), nil
}

// MatchExceptions returns the exceptions in exceptions that apply to the policy
// identified by kind and name. Expired exceptions are never returned: spec.expiresAt
// is documented as retiring an exception, so every path that selects exceptions must
// agree with the admission path about which ones are still live.
func MatchExceptions(exceptions []*policiesv1beta1.PolicyException, kind, name string) []*policiesv1beta1.PolicyException {
	var out []*policiesv1beta1.PolicyException
	for _, exception := range exceptions {
		if exception == nil || exception.IsExpired() {
			continue
		}
		for _, ref := range exception.Spec.PolicyRefs {
			if ref.Name == name && ref.Kind == kind {
				out = append(out, exception)
				break
			}
		}
	}
	return out
}

// NextExceptionExpiry returns how long until the earliest spec.expiresAt among
// exceptions, and false when none of them sets one. An exception that is already
// past its deadline yields one second rather than a non-positive delay, so a
// caller using the result as a requeue delay still gets a prompt requeue.
func NextExceptionExpiry(exceptions []*policiesv1beta1.PolicyException, now time.Time) (time.Duration, bool) {
	var next time.Duration
	found := false
	for _, exception := range exceptions {
		if exception == nil || exception.Spec.ExpiresAt == nil {
			continue
		}
		if until := exception.Spec.ExpiresAt.Sub(now); !found || until < next {
			next, found = until, true
		}
	}
	if found && next <= 0 {
		next = time.Second
	}
	return next, found
}

// ExceptionExpiryResult is the reconcile result for a policy whose cached compiled
// form includes exceptions. The compiled policy only changes on a reconcile, and
// nothing triggers one when wall-clock time passes an exception's expiresAt, so the
// reconcile requeues itself for the earliest deadline; that reconcile lists the
// exceptions again, drops the expired one and recompiles.
func ExceptionExpiryResult(exceptions []*policiesv1beta1.PolicyException) reconcile.Result {
	requeueAfter, ok := NextExceptionExpiry(exceptions, time.Now())
	if !ok {
		return reconcile.Result{}
	}
	return reconcile.Result{RequeueAfter: requeueAfter}
}
