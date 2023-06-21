package controller

import (
	"context"
	"errors"
	"time"

	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type SecretClient interface {
	Informer() cache.SharedIndexInformer
	Lister() corev1listers.SecretLister
}

type secretClient struct {
	informer cache.SharedIndexInformer
	lister   corev1listers.SecretLister
	name     string
}

func (i *secretClient) Informer() cache.SharedIndexInformer {
	return i.informer
}

func (i *secretClient) Lister() corev1listers.SecretLister {
	return i.lister
}

func NewSecretClient(client kubernetes.Interface, namespace, name string) (SecretClient, error) {
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	options := func(lo *metav1.ListOptions) {
		lo.FieldSelector = fields.OneTermEqualSelector(metav1.ObjectNameField, name).String()
	}
	informer := corev1informers.NewFilteredSecretInformer(
		client,
		namespace,
		15*time.Minute,
		indexers,
		options,
	)
	lister := corev1listers.NewSecretLister(informer.GetIndexer())
	ctx := context.TODO()
	go informer.Run(ctx.Done())
	if synced := cache.WaitForCacheSync(ctx.Done(), informer.HasSynced); !synced {
		return nil, errors.New("configmap informer cache failed to sync")
	}
	return &secretClient{
		informer,
		lister,
		name,
	}, nil
}

type CreateClient[T metav1.Object] interface {
	Create(context.Context, T, metav1.CreateOptions) (T, error)
}

type UpdateClient[T metav1.Object] interface {
	Update(context.Context, T, metav1.UpdateOptions) (T, error)
}

type DeleteClient[T metav1.Object] interface {
	Delete(context.Context, string, metav1.DeleteOptions) error
}

type DeleteCollectionClient[T metav1.Object] interface {
	DeleteCollection(context.Context, metav1.DeleteOptions, metav1.ListOptions) error
}

type GetClient[T metav1.Object] interface {
	Get(context.Context, string, metav1.GetOptions) (T, error)
}

type WatchClient[T metav1.Object] interface {
	Watch(context.Context, metav1.ListOptions) (watch.Interface, error)
}

type PatchClient[T metav1.Object] interface {
	Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (T, error)
}

type ObjectClient[T metav1.Object] interface {
	CreateClient[T]
	UpdateClient[T]
	DeleteClient[T]
	DeleteCollectionClient[T]
	GetClient[T]
	WatchClient[T]
	PatchClient[T]
}

type ListClient[T any] interface {
	List(context.Context, metav1.ListOptions) (T, error)
}

type StatusClient[T metav1.Object] interface {
	UpdateStatus(context.Context, T, metav1.UpdateOptions) (T, error)
}

type ObjectListClient[T metav1.Object, L any] interface {
	ObjectClient[T]
	ListClient[L]
}

type ObjectStatusClient[T metav1.Object] interface {
	ObjectClient[T]
	StatusClient[T]
}

type ObjectListStatusClient[T metav1.Object, L any] interface {
	ObjectClient[T]
	ListClient[L]
	StatusClient[T]
}

type Object[T any] interface {
	*T
	metav1.Object
	DeepCopy() *T
}

type Getter[T any] interface {
	Get(string) (T, error)
}

type Setter[T any] interface {
	Create(context.Context, T, metav1.CreateOptions) (T, error)
	Update(context.Context, T, metav1.UpdateOptions) (T, error)
}

type Deleter interface {
	Delete(context.Context, string, metav1.DeleteOptions) error
}

func GetOrNew[T any, R Object[T], G Getter[R]](name string, getter G) (R, error) {
	obj, err := getter.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			obj = new(T)
			obj.SetName(name)
		} else {
			return nil, err
		}
	}
	return obj, nil
}

func CreateOrUpdate[T any, R Object[T], G Getter[R], S Setter[R]](ctx context.Context, name string, getter G, setter S, build func(R) error) (R, error) {
	if obj, err := GetOrNew[T, R](name, getter); err != nil {
		return nil, err
	} else {
		mutated := obj.DeepCopy()
		if err := build(mutated); err != nil {
			return nil, err
		} else {
			if obj.GetResourceVersion() == "" {
				return setter.Create(ctx, mutated, metav1.CreateOptions{})
			} else {
				if datautils.DeepEqual(obj, mutated) {
					return mutated, nil
				} else {
					return setter.Update(ctx, mutated, metav1.UpdateOptions{})
				}
			}
		}
	}
}

type DeepCopy[T any] interface {
	DeepCopy() T
}

func Update[T interface {
	metav1.Object
	DeepCopy[T]
}, S UpdateClient[T]](ctx context.Context, obj T, setter S, build func(T) error,
) (T, error) {
	mutated := obj.DeepCopy()
	if err := build(mutated); err != nil {
		var d T
		return d, err
	} else {
		if datautils.DeepEqual(obj, mutated) {
			return mutated, nil
		} else {
			return setter.Update(ctx, mutated, metav1.UpdateOptions{})
		}
	}
}

func UpdateStatus[T interface {
	metav1.Object
	DeepCopy[T]
}, S StatusClient[T]](ctx context.Context, obj T, setter S, build func(T) error,
) (T, error) {
	mutated := obj.DeepCopy()
	if err := build(mutated); err != nil {
		var d T
		return d, err
	} else {
		if datautils.DeepEqual(obj, mutated) {
			return mutated, nil
		} else {
			return setter.UpdateStatus(ctx, mutated, metav1.UpdateOptions{})
		}
	}
}

func Cleanup[T any, R Object[T]](ctx context.Context, actual []R, expected []R, deleter Deleter) error {
	keep := sets.New[string]()
	for _, obj := range expected {
		keep.Insert(obj.GetName())
	}
	for _, obj := range actual {
		if !keep.Has(obj.GetName()) {
			if err := deleter.Delete(ctx, obj.GetName(), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}
