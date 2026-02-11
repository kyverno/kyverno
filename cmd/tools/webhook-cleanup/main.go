package main

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	managedByLabel      = "webhook.kyverno.io/managed-by"
	managedByLabelValue = "kyverno"
)

func main() {
	// Create Kubernetes client
	config, err := getKubeConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting kubernetes config: %v\n", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating kubernetes client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Delete ValidatingWebhookConfigurations
	if err := deleteValidatingWebhooks(ctx, clientset); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting validating webhooks: %v\n", err)
		os.Exit(1)
	}

	// Delete MutatingWebhookConfigurations
	if err := deleteMutatingWebhooks(ctx, clientset); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting mutating webhooks: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully cleaned up Kyverno webhooks")
}

// getKubeConfig returns the Kubernetes configuration
// It first tries in-cluster config, then falls back to kubeconfig file
func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first (for running inside Kubernetes)
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Log the in-cluster config failure for debugging
	fmt.Fprintf(os.Stderr, "In-cluster config failed (expected in local mode): %v\n", err)

	// Fall back to kubeconfig file (for local development)
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfigPath = home + "/.kube/config"
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, nil
}

// deleteValidatingWebhooks deletes ValidatingWebhookConfigurations managed by Kyverno
func deleteValidatingWebhooks(ctx context.Context, clientset *kubernetes.Clientset) error {
	labelSelector := fmt.Sprintf("%s=%s", managedByLabel, managedByLabelValue)

	// List validating webhooks with the label selector
	webhooks, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list validating webhooks: %w", err)
	}

	if len(webhooks.Items) == 0 {
		fmt.Println("No ValidatingWebhookConfigurations found to delete")
		return nil
	}

	// Delete each validating webhook
	for _, webhook := range webhooks.Items {
		fmt.Printf("Deleting ValidatingWebhookConfiguration: %s\n", webhook.Name)
		err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(
			ctx,
			webhook.Name,
			metav1.DeleteOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to delete validating webhook %s: %w", webhook.Name, err)
		}
		fmt.Printf("Successfully deleted ValidatingWebhookConfiguration: %s\n", webhook.Name)
	}

	return nil
}

// deleteMutatingWebhooks deletes MutatingWebhookConfigurations managed by Kyverno
func deleteMutatingWebhooks(ctx context.Context, clientset *kubernetes.Clientset) error {
	labelSelector := fmt.Sprintf("%s=%s", managedByLabel, managedByLabelValue)

	// List mutating webhooks with the label selector
	webhooks, err := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list mutating webhooks: %w", err)
	}

	if len(webhooks.Items) == 0 {
		fmt.Println("No MutatingWebhookConfigurations found to delete")
		return nil
	}

	// Delete each mutating webhook
	for _, webhook := range webhooks.Items {
		fmt.Printf("Deleting MutatingWebhookConfiguration: %s\n", webhook.Name)
		err := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(
			ctx,
			webhook.Name,
			metav1.DeleteOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to delete mutating webhook %s: %w", webhook.Name, err)
		}
		fmt.Printf("Successfully deleted MutatingWebhookConfiguration: %s\n", webhook.Name)
	}

	return nil
}
