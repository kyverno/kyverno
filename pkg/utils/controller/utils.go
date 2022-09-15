package controller

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type object[T any] interface {
	*T
	metav1.Object
	DeepCopy() *T
}

type getter[T any] interface {
	Get(string) (T, error)
}

type setter[T any] interface {
	Create(context.Context, T, metav1.CreateOptions) (T, error)
	Update(context.Context, T, metav1.UpdateOptions) (T, error)
}

type deleter interface {
	Delete(context.Context, string, metav1.DeleteOptions) error
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

func Cleanup[T any, R object[T]](actual []R, expected []R, deleter deleter) error {
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
