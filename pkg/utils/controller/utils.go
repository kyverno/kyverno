package controller

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type object[K any] interface {
	*K
	metav1.Object
	DeepCopy() *K
}

type getter[K any] interface {
	Get(string) (K, error)
}

type setter[K any] interface {
	Create(context.Context, K, metav1.CreateOptions) (K, error)
	Update(context.Context, K, metav1.UpdateOptions) (K, error)
}

func GetOrNew[T any, R object[T], G getter[R]](name string, getter G) (R, error) {
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

func CreateOrUpdate[T any, R object[T], G getter[R], S setter[R]](name string, getter G, setter S, build func(R) error) (R, error) {
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
