package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestGetObjectWithTombstonePositive(t *testing.T) {
	// Test that the GetObjectWithTombstone function returns the object when given a DeletedFinalStateUnknown object
	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
	}
	tombstone := cache.DeletedFinalStateUnknown{
		Obj: obj,
	}
	result := GetObjectWithTombstone(tombstone)
	if result != obj {
		t.Errorf("Expected GetObjectWithTombstone to return the original object, got %v", result)
	}
}

func TestGetObjectWithTombstoneNegative(t *testing.T) {
	// Test that the GetObjectWithTombstone function returns the original object when not given a DeletedFinalStateUnknown object
	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
	}
	result := GetObjectWithTombstone(obj)
	if result != obj {
		t.Errorf("Expected GetObjectWithTombstone to return the original object, got %v", result)
	}
}

func TestGetObjectWithTombstoneNil(t *testing.T) {
	// Test that the GetObjectWithTombstone function returns nil when given a nil object
	result := GetObjectWithTombstone(nil)
	if result != nil {
		t.Errorf("Expected GetObjectWithTombstone to return nil, got %v", result)
	}
}
