package engine

import (
	"context"
	"slices"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ExceptionScope is the --exceptionNamespace value. "" accepts no exception, "*" accepts every
// namespace, anything else accepts exactly that namespace and treats it as the central one.
type ExceptionScope string

const (
	ExceptionScopeNone ExceptionScope = ""
	ExceptionScopeAll  ExceptionScope = "*"
)

// Applies reports whether polex may apply to the policy identified by kind, name and
// namespace (empty for a cluster-scoped policy).
func (s ExceptionScope) Applies(polex *policiesv1beta1.PolicyException, kind, name, namespace string) bool {
	if polex.IsExpired() {
		return false
	}
	switch s {
	case ExceptionScopeNone:
		// No namespace is trusted to author exceptions.
		return false
	case ExceptionScopeAll:
		// Every namespace may author, but a namespaced policy takes only its own namespace's
		// exceptions, so no namespace can reach another's policies.
		if namespace != "" && polex.GetNamespace() != namespace {
			return false
		}
	default:
		// Only the configured namespace may author. It is also the central namespace, so an
		// exception there may except a policy in any namespace.
		if string(s) != polex.GetNamespace() {
			return false
		}
	}
	return slices.ContainsFunc(polex.Spec.PolicyRefs, func(ref policiesv1beta1.PolicyRef) bool {
		return ref.Name == name && ref.Kind == kind
	})
}

// Select returns the exceptions for which Applies is true, each at most once.
func (s ExceptionScope) Select(exceptions []*policiesv1beta1.PolicyException, kind, name, namespace string) []*policiesv1beta1.PolicyException {
	var out []*policiesv1beta1.PolicyException
	for _, exception := range exceptions {
		if s.Applies(exception, kind, name, namespace) {
			out = append(out, exception)
		}
	}
	return out
}

type PolicyExceptionLister interface {
	// List returns every accepted candidate (used by the reports controller to feed the scanner).
	List(labels.Selector) ([]*policiesv1beta1.PolicyException, error)
	// ListForPolicy returns the exceptions that apply to one policy per ExceptionScope.Applies.
	ListForPolicy(kind, name, namespace string) ([]*policiesv1beta1.PolicyException, error)
}

type policyExceptionLister struct {
	lister policiesv1beta1listers.PolicyExceptionLister
	scope  ExceptionScope
}

func NewPolicyExceptionLister(lister policiesv1beta1listers.PolicyExceptionLister, namespace string) PolicyExceptionLister {
	return &policyExceptionLister{lister: lister, scope: ExceptionScope(namespace)}
}

func (l *policyExceptionLister) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	switch l.scope {
	case ExceptionScopeNone:
		return nil, nil
	case ExceptionScopeAll:
		return l.lister.List(selector)
	default:
		return l.lister.PolicyExceptions(string(l.scope)).List(selector)
	}
}

func (l *policyExceptionLister) ListForPolicy(kind, name, namespace string) ([]*policiesv1beta1.PolicyException, error) {
	exceptions, err := l.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return l.scope.Select(exceptions, kind, name, namespace), nil
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
//
// Filtering by ExceptionScope happens in memory over the result of reader.List, so the
// stale-cache argument above is unchanged: no second informer or cache is introduced.
type managerPolicyExceptionLister struct {
	reader client.Reader
	scope  ExceptionScope
}

// NewManagerPolicyExceptionLister returns a PolicyExceptionLister backed by reader,
// scoped to namespace ("" accepts nothing, "*" accepts every namespace, anything else
// accepts exactly that namespace). Pass the same ctrl.Manager client used to register the
// PolicyException watch that triggers reconciliation, so reads and watch events are served
// from a single cache.
func NewManagerPolicyExceptionLister(reader client.Reader, namespace string) PolicyExceptionLister {
	return &managerPolicyExceptionLister{reader: reader, scope: ExceptionScope(namespace)}
}

func (l *managerPolicyExceptionLister) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	if l.scope == ExceptionScopeNone {
		return nil, nil
	}
	var list policiesv1beta1.PolicyExceptionList
	opts := []client.ListOption{client.MatchingLabelsSelector{Selector: selector}}
	if l.scope != ExceptionScopeAll {
		opts = append(opts, client.InNamespace(string(l.scope)))
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

// AppendExceptions appends to dst every exception in src not already in dst,
// keeping first-seen order. Exceptions are compared by namespace and name.
// Extraction mode collects the exceptions of each pod template into one list,
// and an exception that matches several templates must still be listed once.
// nil entries in src are skipped, and nil entries in dst never match.
func AppendExceptions(dst, src []*policiesv1beta1.PolicyException) []*policiesv1beta1.PolicyException {
	if len(src) == 0 {
		return dst
	}
	// a set of what dst already holds, so each src entry is checked in constant time
	seen := make(map[types.NamespacedName]struct{}, len(dst)+len(src))
	for _, exception := range dst {
		if exception != nil {
			seen[types.NamespacedName{Namespace: exception.GetNamespace(), Name: exception.GetName()}] = struct{}{}
		}
	}
	for _, exception := range src {
		if exception == nil {
			continue
		}
		key := types.NamespacedName{Namespace: exception.GetNamespace(), Name: exception.GetName()}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dst = append(dst, exception)
	}
	return dst
}

func (l *managerPolicyExceptionLister) ListForPolicy(kind, name, namespace string) ([]*policiesv1beta1.PolicyException, error) {
	exceptions, err := l.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return l.scope.Select(exceptions, kind, name, namespace), nil
}
