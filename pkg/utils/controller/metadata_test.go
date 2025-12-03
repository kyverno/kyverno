package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_SetLabel(t *testing.T) {
	obj := &v1.Pod{}
	labels := SetLabel(obj, "foo", "bar")
	assert.Equal(t, "bar", labels["foo"])
	assert.Equal(t, "bar", obj.Labels["foo"])
}

func Test_CheckLabel(t *testing.T) {
	obj := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	}
	assert.True(t, CheckLabel(obj, "foo", "bar"))
	assert.False(t, CheckLabel(obj, "foo", "baz"))
	assert.False(t, CheckLabel(obj, "bar", "foo"))
}

func Test_GetLabel(t *testing.T) {
	obj := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	}
	assert.Equal(t, "bar", GetLabel(obj, "foo"))
	assert.Equal(t, "", GetLabel(obj, "bar"))
}

func Test_SetManagedByKyvernoLabel(t *testing.T) {
	obj := &v1.Pod{}
	SetManagedByKyvernoLabel(obj)
	assert.True(t, IsManagedByKyverno(obj))
}

func Test_IsManagedByKyverno(t *testing.T) {
	obj := &v1.Pod{}
	assert.False(t, IsManagedByKyverno(obj))
	SetManagedByKyvernoLabel(obj)
	assert.True(t, IsManagedByKyverno(obj))
}

func Test_HasLabel(t *testing.T) {
	obj := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	}
	assert.True(t, HasLabel(obj, "foo"))
	assert.False(t, HasLabel(obj, "bar"))
}

func Test_SetAnnotation(t *testing.T) {
	obj := &v1.Pod{}
	SetAnnotation(obj, "foo", "bar")
	assert.Equal(t, "bar", obj.Annotations["foo"])
}

func Test_GetAnnotation(t *testing.T) {
	obj := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"foo": "bar",
			},
		},
	}
	assert.Equal(t, "bar", GetAnnotation(obj, "foo"))
	assert.Equal(t, "", GetAnnotation(obj, "bar"))
}

func Test_HasAnnotation(t *testing.T) {
	obj := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"foo": "bar",
			},
		},
	}
	assert.True(t, HasAnnotation(obj, "foo"))
	assert.False(t, HasAnnotation(obj, "bar"))
}

func Test_SetOwner(t *testing.T) {
	obj := &v1.Pod{}
	SetOwner(obj, "v1", "Pod", "foo", "12345")
	assert.Equal(t, "v1", obj.OwnerReferences[0].APIVersion)
	assert.Equal(t, "Pod", obj.OwnerReferences[0].Kind)
	assert.Equal(t, "foo", obj.OwnerReferences[0].Name)
	assert.Equal(t, "12345", string(obj.OwnerReferences[0].UID))
}
