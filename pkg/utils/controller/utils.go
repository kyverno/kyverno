package controller

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
)

type ClientCreate[T metav1.Object] interface {
	Create(context.Context, T, metav1.CreateOptions) (T, error)
}

type ClientUpdate[T metav1.Object] interface {
	Update(context.Context, T, metav1.UpdateOptions) (T, error)
}

type ClientDelete[T metav1.Object] interface {
	Delete(context.Context, string, metav1.DeleteOptions) error
}

type ClientDeleteCollection[T metav1.Object] interface {
	DeleteCollection(context.Context, metav1.DeleteOptions, metav1.ListOptions) error
}

type ClientGet[T metav1.Object] interface {
	Get(context.Context, string, metav1.GetOptions) (T, error)
}

type ClientWatch[T metav1.Object] interface {
	Watch(context.Context, metav1.ListOptions) (watch.Interface, error)
}

type ClientPatch[T metav1.Object] interface {
	Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (T, error)
}

type ClientList[T any] interface {
	List(context.Context, metav1.ListOptions) (T, error)
}

type Client[T metav1.Object, L any] interface {
	ClientCreate[T]
	ClientUpdate[T]
	ClientDelete[T]
	ClientDeleteCollection[T]
	ClientGet[T]
	ClientWatch[T]
	ClientPatch[T]
	ClientList[L]
}

type StatusClient[T metav1.Object] interface {
	UpdateStatus(context.Context, T, metav1.UpdateOptions) (T, error)
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

func CreateOrUpdate[T any, R Object[T], G Getter[R], S Setter[R]](name string, getter G, setter S, build func(R) error) (R, error) {
	if obj, err := GetOrNew[T, R](name, getter); err != nil {
		return nil, err
	} else {
		mutated := obj.DeepCopy()
		if err := build(mutated); err != nil {
			return nil, err
		} else {
			if obj.GetResourceVersion() == "" {
				return setter.Create(context.TODO(), mutated, metav1.CreateOptions{})
			} else {
				if reflect.DeepEqual(obj, mutated) {
					return mutated, nil
				} else {
					return setter.Update(context.TODO(), mutated, metav1.UpdateOptions{})
				}
			}
		}
	}
}

func Update[T any, R Object[T], S Setter[R]](setter S, obj R, build func(R) error) (R, error) {
	mutated := obj.DeepCopy()
	if err := build(mutated); err != nil {
		return nil, err
	} else {
		if reflect.DeepEqual(obj, mutated) {
			return mutated, nil
		} else {
			return setter.Update(context.TODO(), mutated, metav1.UpdateOptions{})
		}
	}
}

func Cleanup[T any, R Object[T]](actual []R, expected []R, deleter Deleter) error {
	keep := sets.NewString()
	for _, obj := range expected {
		keep.Insert(obj.GetName())
	}
	for _, obj := range actual {
		if !keep.Has(obj.GetName()) {
			if err := deleter.Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}
