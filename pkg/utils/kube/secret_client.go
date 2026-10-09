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

// ScopeSecretLister enables authorized single-Secret GETs for a policy's own
// namespace. It does not start additional informers or list tenant secrets.
func ScopeSecretLister(lister corev1listers.SecretLister, namespace string) corev1listers.SecretLister {
	if namespace == "" {
		return lister
	}
	if provider, ok := lister.(interface {
		ForNamespace(string) corev1listers.SecretLister
	}); ok {
		return provider.ForNamespace(namespace)
	}
	return lister
}

// ScopeSecretListerWithContext binds credential callbacks to the
// caller's context while retaining the same namespace boundary.
func ScopeSecretListerWithContext(ctx context.Context, lister corev1listers.SecretLister, namespace string) corev1listers.SecretLister {
	if namespace == "" {
		return lister
	}
	if provider, ok := lister.(interface {
		ForNamespaceWithContext(context.Context, string) corev1listers.SecretLister
	}); ok {
		return provider.ForNamespaceWithContext(ctx, namespace)
	}
	return ScopeSecretLister(lister, namespace)
}

func (l *secretClientLister) ForNamespaceWithContext(ctx context.Context, namespace string) corev1listers.SecretLister {
	scoped := *l
	scoped.ctx = ctx
	return scoped.ForNamespace(namespace)
}

func (l *secretClientLister) ForNamespace(namespace string) corev1listers.SecretLister {
	return &scopedSecretLister{owner: l, namespace: namespace}
}

type scopedSecretLister struct {
	owner     *secretClientLister
	namespace string
}

func (l *scopedSecretLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	return l.owner.Secrets(l.namespace).List(selector)
}

func (l *scopedSecretLister) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	return &scopedSecretNamespaceLister{owner: l.owner, namespace: namespace, policyNamespace: l.namespace}
}

type scopedSecretNamespaceLister struct {
	owner                      *secretClientLister
	namespace, policyNamespace string
}

func (l *scopedSecretNamespaceLister) scopeError(name string) error {
	if l.namespace == l.policyNamespace {
		return nil
	}
	return apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, name, fmt.Errorf("secret namespace %q does not match policy namespace %q", l.namespace, l.policyNamespace))
}

func (l *scopedSecretNamespaceLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	if err := l.scopeError(""); err != nil {
		return nil, err
	}
	return l.owner.Secrets(l.namespace).List(selector)
}

func (l *scopedSecretNamespaceLister) Get(name string) (*corev1.Secret, error) {
	return l.GetWithContext(l.owner.ctx, name)
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
	secret, err := l.owner.Secrets(l.namespace).Get(name)
	if !apierrors.IsNotFound(err) {
		return secret, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return l.owner.client.Secrets(l.namespace).Get(ctx, name, metav1.GetOptions{})
}
