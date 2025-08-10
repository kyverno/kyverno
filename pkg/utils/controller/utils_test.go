package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockDeleter struct {
	deleted []string
}

func (d *mockDeleter) Delete(ctx context.Context, name string, options metav1.DeleteOptions) error {
	d.deleted = append(d.deleted, name)
	return nil
}

type mockObject struct {
	metav1.ObjectMeta
}

func (o *mockObject) DeepCopy() *mockObject {
	return &mockObject{
		ObjectMeta: *o.ObjectMeta.DeepCopy(),
	}
}

func TestCleanup(t *testing.T) {
	deleter := &mockDeleter{}
	actual := []*mockObject{
		{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "bar"}},
	}
	expected := []*mockObject{
		{ObjectMeta: metav1.ObjectMeta{Name: "bar"}},
	}
	err := Cleanup(context.Background(), actual, expected, deleter)
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo"}, deleter.deleted)
}
