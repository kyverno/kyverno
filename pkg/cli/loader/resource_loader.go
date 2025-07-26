package loader

import (
	"context"
	"fmt"
	"time"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const avgSequentialCallTime = 300 * time.Millisecond

func LoadResourcesConcurrent(policies []engineapi.GenericPolicy, dClient dclient.Interface, resourceOptions ResourceOptions, showPerformance bool) ([]*unstructured.Unstructured, error) {
	resourceLoader, err := NewClusterLoader(dClient, resourceOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource loader: %w", err)
	}
	defer resourceLoader.Close()

	result, err := resourceLoader.LoadResources(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to load resources: %w", err)
	}

	if showPerformance {
		printLoadingSummary(result.Report)
	}

	return result.Resources, nil
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

	sequentialEstimate := time.Duration(report.APICallsCount) * avgSequentialCallTime
	if sequentialEstimate > report.Duration {
		improvement := float64(sequentialEstimate-report.Duration) / float64(sequentialEstimate) * 100
		fmt.Printf("  Estimated improvement: %.1f%% faster than sequential\n", improvement)
	}
}
