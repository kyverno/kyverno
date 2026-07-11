package main

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCreatePodsSuccess(t *testing.T) {
	client := kubefake.NewSimpleClientset()
	namespace = "test"

	errs := createPods(context.Background(), client, namespace, 3)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d: %v", len(errs), errs)
	}

	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list pods: %v", err)
	}
	if len(pods.Items) != 3 {
		t.Fatalf("expected 3 pods, got %d", len(pods.Items))
	}
}

func TestCreatePodsCollectsPerPodErrors(t *testing.T) {
	client := kubefake.NewSimpleClientset()
	namespace = "test"

	failedPods := map[string]error{
		"perf-testing-pod-1": errors.New("boom-1"),
		"perf-testing-pod-3": errors.New("boom-3"),
	}
	client.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		pod := createAction.GetObject().(*corev1.Pod)
		if err, ok := failedPods[pod.Name]; ok {
			return true, nil, err
		}
		return false, nil, nil
	})

	errs := createPods(context.Background(), client, namespace, 4)
	if len(errs) != len(failedPods) {
		t.Fatalf("expected %d errors, got %d: %v", len(failedPods), len(errs), errs)
	}

	got := make([]string, 0, len(errs))
	for _, err := range errs {
		got = append(got, err.Error())
	}
	sort.Strings(got)

	want := []string{
		"perf-testing-pod-1: boom-1",
		"perf-testing-pod-3: boom-3",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected errors:\nwant:\n%s\n\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}
