package controller

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockEnqueuer struct {
	enqueued []interface{}
}

func (e *mockEnqueuer) Enqueue(obj interface{}) error {
	e.enqueued = append(e.enqueued, obj)
	return nil
}

func TestAddFunc(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	addFunc := AddFunc(logr.Discard(), enqueuer.Enqueue)
	obj := &metav1.ObjectMeta{ResourceVersion: "1"}
	addFunc(obj)
	assert.Equal(t, []interface{}{obj}, enqueuer.enqueued)
}

func TestUpdateFunc(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	updateFunc := UpdateFunc(logr.Discard(), enqueuer.Enqueue)

	// same resource version
	updateFunc(&metav1.ObjectMeta{ResourceVersion: "1"}, &metav1.ObjectMeta{ResourceVersion: "1"})
	assert.Empty(t, enqueuer.enqueued)

	// different resource version
	obj := &metav1.ObjectMeta{ResourceVersion: "2"}
	updateFunc(&metav1.ObjectMeta{ResourceVersion: "1"}, obj)
	assert.Equal(t, []interface{}{obj}, enqueuer.enqueued)
}

func TestDeleteFunc(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	deleteFunc := DeleteFunc(logr.Discard(), enqueuer.Enqueue)
	obj := &metav1.ObjectMeta{ResourceVersion: "1"}
	deleteFunc(obj)
	assert.Equal(t, []interface{}{obj}, enqueuer.enqueued)
}