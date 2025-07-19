package loader

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func LoadResourcesConcurrent(policies []engineapi.GenericPolicy, dClient dclient.Interface, resourceOptions ResourceOptions, showPerformance bool) ([]*unstructured.Unstructured, error) {

	resourceLoader, err := createResourceLoader(dClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource loader: %w", err)
	}
	defer resourceLoader.Close()

	result, err := resourceLoader.LoadResources(context.Background(), ResourceOptions{
		Namespace:       resourceOptions.Namespace,
		ResourceTypes:   resourceOptions.ResourceTypes,
		Concurrency:     getConcurrency(resourceOptions.Concurrency),
		BatchSize:       getBatchSize(resourceOptions.BatchSize),
		ContinueOnError: resourceOptions.ContinueOnError,
		Timeout:         5 * time.Minute,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load resources: %w", err)
	}

	if showPerformance {
		printLoadingSummary(result.Report)
	}

	return result.Resources, nil
}

func createResourceLoader(dClient dclient.Interface) (ResourceLoader, error) {
	return NewClusterLoader(
		dClient.GetDynamicInterface(),
		ClusterLoaderConfig{
			DefaultConcurrency: 4,
			DefaultBatchSize:   100,
			DefaultTimeout:     5 * time.Minute,
			DefaultMaxRetries:  3,
		})
}

func getConcurrency(Concurrency int) int {
	if Concurrency > 0 {
		return Concurrency
	}

	concurrency := runtime.NumCPU()
	if concurrency > 8 {
		concurrency = 8
	}
	return concurrency
}

func getBatchSize(BatchSize int) int {
	if BatchSize > 0 {
		return BatchSize
	}
	return 100
}

func printLoadingSummary(report LoadReport) {
	fmt.Printf("\nResource Loading Performance:\n")
	fmt.Printf("  Duration: %s\n", report.Duration)
	fmt.Printf("  Resources loaded: %d\n", report.SuccessfullyLoaded)
	fmt.Printf("  API calls: %d\n", report.APICallsCount)
	fmt.Printf("  Concurrent workers: %d\n", report.ConcurrentWorkers)

	if len(report.Errors) > 0 {
		fmt.Printf("  Errors: %d\n", len(report.Errors))
		for _, err := range report.Errors {
			fmt.Printf("    - %s: %s\n", err.ResourceType, err.Error)
		}
	}

	sequentialEstimate := time.Duration(report.APICallsCount) * 300 * time.Millisecond
	if sequentialEstimate > report.Duration {
		improvement := float64(sequentialEstimate-report.Duration) / float64(sequentialEstimate) * 100
		fmt.Printf("  Estimated improvement: %.1f%% faster than sequential\n", improvement)
	}
}
