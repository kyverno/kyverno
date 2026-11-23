package kube

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// secretGetTimeout bounds the authorized live GET so a credential lookup cannot
// hold an admission request open on a slow API server.
const secretGetTimeout = 5 * time.Second

// WithSecretClient attaches a client for explicit namespace-scoped credential
// lookups. Ordinary lister calls retain the existing informer behavior.
func WithSecretClient(ctx context.Context, cached corev1listers.SecretLister, client corev1client.SecretsGetter) corev1listers.SecretLister {
	return &secretClientLister{SecretLister: cached, ctx: ctx, client: client}
}

type secretClientLister struct {
	corev1listers.SecretLister
	ctx    context.Context //nolint:containedctx
	client corev1client.SecretsGetter
}

func (l *secretClientLister) ForNamespace(namespace string) corev1listers.SecretLister {
	return l.ForNamespaceWithContext(l.ctx, namespace)
}

func (l *secretClientLister) ForNamespaceWithContext(ctx context.Context, namespace string) corev1listers.SecretLister {
	return &scopedSecretLister{cache: l.SecretLister, client: l.client, ctx: ctx, namespace: namespace}
}

// ScopeSecretLister confines a lister to a single namespace. A lister built by
// WithSecretClient also gains authorized single-Secret GETs for that namespace;
// any other lister is confined cache-only rather than returned unscoped, so a
// caller that forgets the wrapper loses the live GET but never the namespace
// boundary. It does not start additional informers or list tenant secrets.
//
// An empty namespace means the caller is not namespace-confined at all
// (cluster-scoped ImageValidatingPolicy, operator-configured global
// credentials) and the lister is returned untouched.
// Scoping is idempotent. compilerImpl.Compile scopes the lister and then hands
// the result to NewIvFuncs, which scopes it again for the same namespace.
// Wrapping twice would make the outer GetWithContext delegate through the inner
// lister's plain Get and silently drop the request context, so the authorized
// GET would run under the process context instead of the admission request's.
func ScopeSecretLister(lister corev1listers.SecretLister, namespace string) corev1listers.SecretLister {
	if namespace == "" || lister == nil {
		return lister
	}
	if scoped, ok := lister.(*scopedSecretLister); ok && scoped.namespace == namespace {
		return scoped
	}
	if provider, ok := lister.(interface {
		ForNamespace(string) corev1listers.SecretLister
	}); ok {
		return provider.ForNamespace(namespace)
	}
	return &scopedSecretLister{cache: lister, ctx: context.Background(), namespace: namespace}
}

// ScopeSecretListerWithContext binds credential callbacks to the
// caller's context while retaining the same namespace boundary.
func ScopeSecretListerWithContext(ctx context.Context, lister corev1listers.SecretLister, namespace string) corev1listers.SecretLister {
	if namespace == "" || lister == nil {
		return lister
	}
	if provider, ok := lister.(interface {
		ForNamespaceWithContext(context.Context, string) corev1listers.SecretLister
	}); ok {
		return provider.ForNamespaceWithContext(ctx, namespace)
	}
	if scoped, ok := lister.(*scopedSecretLister); ok && scoped.namespace == namespace {
		rebound := *scoped
		rebound.ctx = ctx
		return &rebound
	}
	return &scopedSecretLister{cache: lister, ctx: ctx, namespace: namespace}
}

// CredentialNamespace reports the namespace a scoped lister is confined to, so
// callers that resolve a bare Secret name default to the policy's own namespace
// instead of Kyverno's. fallback is returned when the lister is not confined,
// which is the cluster-scoped and global-credential case.
//
// Deriving this from the lister rather than passing it separately keeps the two
// from drifting: the namespace used to resolve a bare name is by construction
// the namespace the lookup is allowed to read.
func CredentialNamespace(lister corev1listers.SecretLister, fallback string) string {
	if scoped, ok := lister.(interface{ ScopedNamespace() string }); ok {
		if namespace := scoped.ScopedNamespace(); namespace != "" {
			return namespace
		}
	}
	return fallback
}

// scopedSecretLister confines every lookup to one namespace. client may be nil,
// in which case lookups stay cache-only and a cache miss stays NotFound.
type scopedSecretLister struct {
	cache     corev1listers.SecretLister
	client    corev1client.SecretsGetter
	ctx       context.Context //nolint:containedctx
	namespace string
}

// ScopedNamespace implements the probe CredentialNamespace uses.
func (l *scopedSecretLister) ScopedNamespace() string {
	return l.namespace
}

func (l *scopedSecretLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	return l.Secrets(l.namespace).List(selector)
}

func (l *scopedSecretLister) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	return &scopedSecretNamespaceLister{parent: l, namespace: namespace}
}

type scopedSecretNamespaceLister struct {
	parent    *scopedSecretLister
	namespace string
}

func (l *scopedSecretNamespaceLister) scopeError(name string) error {
	if l.namespace == l.parent.namespace {
		return nil
	}
	return apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, name, fmt.Errorf("secret namespace %q does not match policy namespace %q", l.namespace, l.parent.namespace))
}

func (l *scopedSecretNamespaceLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	if err := l.scopeError(""); err != nil {
		return nil, err
	}
	return l.parent.cache.Secrets(l.namespace).List(selector)
}

func (l *scopedSecretNamespaceLister) Get(name string) (*corev1.Secret, error) {
	return l.GetWithContext(l.parent.ctx, name)
}

// GetWithContext lets context-aware credential clients propagate their request
// deadline. Older clients calling Get still have a bounded live API request.
func (l *scopedSecretNamespaceLister) GetWithContext(ctx context.Context, name string) (*corev1.Secret, error) {
	if err := l.scopeError(name); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Hand the request context down when the underlying lister understands it,
	// so a nested scope cannot degrade a live GET to the stored context.
	cache := l.parent.cache.Secrets(l.namespace)
	var secret *corev1.Secret
	var err error
	if contextual, ok := cache.(interface {
		GetWithContext(context.Context, string) (*corev1.Secret, error)
	}); ok {
		secret, err = contextual.GetWithContext(ctx, name)
	} else {
		secret, err = cache.Get(name)
	}
	// Only a cache miss escalates to the API server, and only when a client was
	// configured. Forbidden and every other error stay authoritative.
	if !apierrors.IsNotFound(err) || l.parent.client == nil {
		return secret, err
	}
	ctx, cancel := context.WithTimeout(ctx, secretGetTimeout)
	defer cancel()
	return l.parent.client.Secrets(l.namespace).Get(ctx, name, metav1.GetOptions{})
}
