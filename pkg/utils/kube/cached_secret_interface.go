package kube

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var secretResource = schema.GroupResource{Resource: "secrets"}

var _ k8scorev1.SecretInterface = (*cachedSecretInterface)(nil)

type cachedSecretInterface struct {
	lister    corev1listers.SecretLister
	namespace string
}

// NewCachedSecretInterface adapts a SecretLister to the typed SecretInterface used by image loading code.
func NewCachedSecretInterface(lister corev1listers.SecretLister, namespace string) k8scorev1.SecretInterface {
	return &cachedSecretInterface{
		lister:    lister,
		namespace: namespace,
	}
}

func (c *cachedSecretInterface) Get(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
	if c.lister == nil {
		return nil, errors.New("secret lister is nil")
	}
	return c.lister.Secrets(c.namespace).Get(name)
}

func (c *cachedSecretInterface) List(_ context.Context, opts metav1.ListOptions) (*corev1.SecretList, error) {
	if c.lister == nil {
		return nil, errors.New("secret lister is nil")
	}
	selector := labels.Everything()
	if opts.LabelSelector != "" {
		parsed, err := labels.Parse(opts.LabelSelector)
		if err != nil {
			return nil, err
		}
		selector = parsed
	}
	secrets, err := c.lister.Secrets(c.namespace).List(selector)
	if err != nil {
		return nil, err
	}
	list := &corev1.SecretList{
		Items: make([]corev1.Secret, 0, len(secrets)),
	}
	for _, secret := range secrets {
		list.Items = append(list.Items, *secret)
	}
	return list, nil
}

func (c *cachedSecretInterface) Create(context.Context, *corev1.Secret, metav1.CreateOptions) (*corev1.Secret, error) {
	return nil, apierrors.NewMethodNotSupported(secretResource, "create")
}

func (c *cachedSecretInterface) Update(context.Context, *corev1.Secret, metav1.UpdateOptions) (*corev1.Secret, error) {
	return nil, apierrors.NewMethodNotSupported(secretResource, "update")
}

func (c *cachedSecretInterface) Delete(context.Context, string, metav1.DeleteOptions) error {
	return apierrors.NewMethodNotSupported(secretResource, "delete")
}

func (c *cachedSecretInterface) DeleteCollection(context.Context, metav1.DeleteOptions, metav1.ListOptions) error {
	return apierrors.NewMethodNotSupported(secretResource, "deletecollection")
}

func (c *cachedSecretInterface) Watch(context.Context, metav1.ListOptions) (watch.Interface, error) {
	return nil, apierrors.NewMethodNotSupported(secretResource, "watch")
}

func (c *cachedSecretInterface) Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*corev1.Secret, error) {
	return nil, apierrors.NewMethodNotSupported(secretResource, "patch")
}

func (c *cachedSecretInterface) Apply(context.Context, *corev1apply.SecretApplyConfiguration, metav1.ApplyOptions) (*corev1.Secret, error) {
	return nil, apierrors.NewMethodNotSupported(secretResource, "apply")
}
