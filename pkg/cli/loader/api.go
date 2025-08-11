package loader

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceLoader interface {
	LoadResources(ctx context.Context) (*ResourceResult, error)
}

type ResourceOptions struct {
	Namespace     string
	LabelSelector string
	FieldSelector string
	ResourceTypes []schema.GroupVersionKind

	Concurrency     int
	BatchSize       int
	Timeout         time.Duration
	ContinueOnError bool
	MaxRetries      int
}

type ResourceResult struct {
	Resources []*unstructured.Unstructured
	Report    LoadReport
}

type LoadReport struct {
	TotalRequested     int
	SuccessfullyLoaded int
	Failed             int
	Duration           time.Duration
	Errors             []LoadError
	APICallsCount      int
	ConcurrentWorkers  int
}

type LoadError struct {
	ResourceType string
	Namespace    string
	Error        error
	Retryable    bool
	Timestamp    time.Time
}
