package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var errNoReadyEndpoints = errors.New("no ready endpoints")

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: readiness-checker <command> [flags]")
		fmt.Println("Commands:")
		fmt.Println("  check-endpoints    Check if reports server endpoints are ready")
		fmt.Println("  check-http      	  Check HTTP endpoint availability")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "check-endpoints":
		runCheckEndpoints()
	case "check-http":
		runCheckHTTP()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Available commands: check-endpoints, check-metrics")
		os.Exit(1)
	}
}

func runCheckEndpoints() {
	var (
		serviceName string
		namespace   string
		timeout     time.Duration
	)

	fs := flag.NewFlagSet("check-endpoints", flag.ExitOnError)
	fs.StringVar(&serviceName, "service-name", "", "Service name")
	fs.StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	fs.DurationVar(&timeout, "timeout", 300*time.Second, "Timeout duration")
	err := fs.Parse(os.Args[2:])
	if err != nil {
		fmt.Printf("error parsing flags: %s", err.Error())
		os.Exit(1)
	}

	if namespace == "" {
		fmt.Println("Error: --namespace is required")
		os.Exit(1)
	}
	if serviceName == "" {
		fmt.Println("Error: --service-name is required")
		os.Exit(1)
	}

	clientset, err := getKubernetesClient()
	if err != nil {
		fmt.Printf("Failed to create Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var existingEndpointSliceNames []string

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Timeout reached after %s. service %s is not ready.\n", serviceName, timeout)
			os.Exit(1)
		default:
			err := attemptCheckEndpoints(ctx, clientset, serviceName, namespace, existingEndpointSliceNames)
			if err != nil {
				if err == errNoReadyEndpoints {
					fmt.Println("failed to find a ready endpoint, sleeping for 5 seconds")
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

func runCheckHTTP() {
	var (
		serviceName string
		namespace   string
		path        string
		https       bool
		port        int
		timeout     time.Duration
	)

	fs := flag.NewFlagSet("check-http", flag.ExitOnError)
	fs.StringVar(&serviceName, "service-name", "", "Service name")
	fs.StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	fs.StringVar(&path, "path", "", "The endpoint path")
	fs.BoolVar(&https, "https", false, "Use HTTPS in the request")
	fs.IntVar(&port, "port", 8000, "Service port")
	fs.DurationVar(&timeout, "timeout", 60*time.Second, "HTTP request timeout")
	err := fs.Parse(os.Args[2:])
	if err != nil {
		fmt.Printf("error parsing flags: %s", err.Error())
		os.Exit(1)
	}

	if serviceName == "" {
		fmt.Println("Error: --service-name is required")
		os.Exit(1)
	}
	if namespace == "" {
		fmt.Println("Error: --namespace is required")
		os.Exit(1)
	}

	url := fmt.Sprintf("http://%s.%s:%d/%s", serviceName, namespace, port, path)
	if https {
		url = fmt.Sprintf("https://%s.%s:%d/%s", serviceName, namespace, port, path)
	}

	fmt.Printf("Checking endpoint: %s\n", url)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("error building request: %s", err.Error())
		os.Exit(1)
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("timeout waiting for endpoint %s to become ready\n", url)
			os.Exit(1)
		default:
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("Failed to fetch: %s\n", err.Error())
				time.Sleep(time.Second * 5)
				continue
			}

			defer resp.Body.Close()
			fmt.Printf("HTTP Status: %s\n", resp.Status)
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("Endpoint returned non-OK status: %d\n", resp.StatusCode)
				time.Sleep(time.Second * 5)
				continue
			}
			return
		}
	}
}

func attemptCheckEndpoints(ctx context.Context, clientset *kubernetes.Clientset, svcName, namespace string, existingEndpointSliceNames []string) error {
	if existingEndpointSliceNames == nil {
		endpointSlices, err := clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, e := range endpointSlices.Items {
			for _, owner := range e.OwnerReferences {
				if owner.Kind == "Service" && owner.Name == svcName {
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
