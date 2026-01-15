package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	var (
		releaseName string
		namespace   string
		timeout     int
	)

	flag.StringVar(&releaseName, "release-name", "", "Helm release name")
	flag.StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	flag.IntVar(&timeout, "timeout", 300, "Timeout in seconds")
	flag.Parse()

	if releaseName == "" {
		fmt.Println("Error: --release-name is required")
		os.Exit(1)
	}

	if namespace == "" {
		fmt.Println("Error: --namespace is required")
		os.Exit(1)
	}

	clientset, err := getKubernetesClient()
	if err != nil {
		fmt.Printf("Failed to create Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	endpointName := fmt.Sprintf("%s-reports-server", releaseName)
	ctx := context.Background()
	elapsed := 0

	for elapsed < timeout {
		endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(ctx, endpointName, metav1.GetOptions{})
		if err == nil {
			if len(endpoints.Subsets) > 0 {
				for _, subset := range endpoints.Subsets {
					if len(subset.Addresses) > 0 {
						fmt.Println("Reports-server is ready!")
						os.Exit(0)
					}
				}
			}
		}

		fmt.Printf("Waiting for reports-server... (%d/%d seconds)\n", elapsed, timeout)
		time.Sleep(5 * time.Second)
		elapsed += 5
	}

	fmt.Printf("Timeout reached after %d seconds. Reports-server is not ready.\n", timeout)
	os.Exit(1)
}

func getKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to initalize kube client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, nil
}
