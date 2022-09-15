package controller

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func SetLabel(obj metav1.Object, key, value string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[key] = value
	obj.SetLabels(labels)
}

func SetAnnotation(obj metav1.Object, key, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	obj.SetAnnotations(annotations)
}

func SetOwner(obj metav1.Object, apiVersion, kind, name string, uid types.UID) {
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        uid,
	}})
}

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

func GetOrNew[T any, R object[T], G getter[R]](name string, getter G, build func(R) error) (R, error) {
	obj, err := getter.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			obj = new(T)
			obj.SetName(name)
		} else {
			return nil, err
		}
	} else {
		obj = obj.DeepCopy()
	}
	if err := build(obj); err != nil {
		return nil, err
	} else {
		return obj, nil
	}
}

func CreateOrUpdateFunc[T any, R object[T], G getter[R], S setter[R]](getter G, setter S) func(string, func(R) error) (R, error) {
	return func(name string, build func(R) error) (R, error) {
		return CreateOrUpdate(name, getter, setter, build)
	}
}

func CreateOrUpdate[T any, R object[T], G getter[R], S setter[R]](name string, getter G, setter S, build func(R) error) (R, error) {
	if obj, err := GetOrNew(name, getter, build); err != nil {
		return nil, err
	} else {
		if obj.GetResourceVersion() == "" {
			return setter.Create(context.TODO(), obj, metav1.CreateOptions{})
		} else {
			return setter.Update(context.TODO(), obj, metav1.UpdateOptions{})
		}
	}
}
