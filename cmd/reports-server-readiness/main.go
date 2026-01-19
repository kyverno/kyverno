package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	releaseName                string
	namespace                  string
	timeout                    time.Duration
	existingEndpointSliceNames []string
	errNoReadyEndpoints        = errors.New("no ready endpoints")
)

func main() {
	flag.StringVar(&releaseName, "release-name", "kyverno", "Helm release name")
	flag.StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	flag.DurationVar(&timeout, "timeout", 300*time.Second, "Timeout duration")
	flag.Parse()

	if namespace == "" {
		fmt.Println("Error: --namespace is required")
		os.Exit(1)
	}
	if releaseName == "" {
		fmt.Println("Error: --release-name is required")
		os.Exit(1)
	}

	clientset, err := getKubernetesClient()
	if err != nil {
		fmt.Printf("Failed to create Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Timeout reached after %s. Reports-server is not ready.\n", timeout)
			os.Exit(1)
		default:
			err := attemptCheckReportsServer(ctx, clientset)
			if err != nil {
				if err == errNoReadyEndpoints {
					fmt.Println("failed to find a ready endpoint for the reports server, sleeping for 5 seconds")
					time.Sleep(5 * time.Second)
					continue
				}
				panic(err)
			}

			fmt.Println("reports server is ready!")
			return
		}
	}
}

func getKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kube client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, nil
}

func attemptCheckReportsServer(ctx context.Context, clientset *kubernetes.Clientset) error {
	if existingEndpointSliceNames == nil {
		endpointSlices, err := clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, e := range endpointSlices.Items {
			for _, owner := range e.OwnerReferences {
				if owner.Kind == "Service" && owner.Name == fmt.Sprintf("%s-reports-server", releaseName) {
					// we are ready, no need to do further processing
					for _, endpoint := range e.Endpoints {
						if *endpoint.Conditions.Ready {
							return nil
						}
					}

					// we aren't ready, need to store the endpoints to later fetch them with Get
					if existingEndpointSliceNames == nil {
						existingEndpointSliceNames = []string{}
					}
					for _, existing := range existingEndpointSliceNames {
						if e.Name == existing {
							continue
						}
					}
					existingEndpointSliceNames = append(existingEndpointSliceNames, e.Name)
				}
			}
		}
		return errNoReadyEndpoints
	}
	// we had existing endpoints from the previous list call. get those again and check if they became ready
	for _, existingEps := range existingEndpointSliceNames {
		eps, err := clientset.DiscoveryV1().EndpointSlices(namespace).Get(ctx, existingEps, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Error fetching endpoint %s: %s", eps, err.Error())
			continue
		}
		for _, endpoint := range eps.Endpoints {
			if *endpoint.Conditions.Ready {
				return nil
			}
		}
	}

	return errNoReadyEndpoints
}
