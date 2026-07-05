package registryclient

import (
	"context"
	"errors"
	"fmt"

	kyvernoconfig "github.com/kyverno/kyverno/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	applyconfigurationscorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var (
	errReadOnlySecretInterface = errors.New("read-only secret interface backed by informer cache")
)

type secretInterfaceFromLister struct {
	lister           corev1listers.SecretLister
	defaultNamespace string
}

var _ k8scorev1.SecretInterface = (*secretInterfaceFromLister)(nil)

// SecretInterfaceFromLister returns a read-only SecretInterface that resolves secrets from
// an informer cache instead of the API server.
func SecretInterfaceFromLister(lister corev1listers.SecretLister, defaultNamespace string) k8scorev1.SecretInterface {
	return &secretInterfaceFromLister{
		lister:           lister,
		defaultNamespace: defaultNamespace,
	}
}

// CachedSecretInterface returns a SecretInterface backed by the informer cache using the
// Kyverno install namespace as the default secret namespace.
func CachedSecretInterface(lister corev1listers.SecretLister) k8scorev1.SecretInterface {
	return SecretInterfaceFromLister(lister, kyvernoconfig.KyvernoNamespace())
}

func (s *secretInterfaceFromLister) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Secret, error) {
	if s.lister == nil {
		return nil, fmt.Errorf("secret lister is nil, cannot get secret %q", name)
	}
	namespace, secretName := ParseSecretReference(name, s.defaultNamespace)
	return s.lister.Secrets(namespace).Get(secretName)
}

func (s *secretInterfaceFromLister) Create(context.Context, *corev1.Secret, metav1.CreateOptions) (*corev1.Secret, error) {
	return nil, errReadOnlySecretInterface
}

func (s *secretInterfaceFromLister) Update(context.Context, *corev1.Secret, metav1.UpdateOptions) (*corev1.Secret, error) {
	return nil, errReadOnlySecretInterface
}

func (s *secretInterfaceFromLister) Delete(context.Context, string, metav1.DeleteOptions) error {
	return errReadOnlySecretInterface
}

func (s *secretInterfaceFromLister) DeleteCollection(context.Context, metav1.DeleteOptions, metav1.ListOptions) error {
	return errReadOnlySecretInterface
}

func (s *secretInterfaceFromLister) List(context.Context, metav1.ListOptions) (*corev1.SecretList, error) {
	return nil, errReadOnlySecretInterface
}

func (s *secretInterfaceFromLister) Watch(context.Context, metav1.ListOptions) (watch.Interface, error) {
	return nil, errReadOnlySecretInterface
}

func (s *secretInterfaceFromLister) Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*corev1.Secret, error) {
	return nil, errReadOnlySecretInterface
}

func (s *secretInterfaceFromLister) Apply(context.Context, *applyconfigurationscorev1.SecretApplyConfiguration, metav1.ApplyOptions) (*corev1.Secret, error) {
	return nil, errReadOnlySecretInterface
}
