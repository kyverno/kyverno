package controller

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

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
