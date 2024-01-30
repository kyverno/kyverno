package sysdump

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type client struct {
	kubernetesClientSet *kubernetes.Clientset
	kyvernoClientSet    *kyverno.Clientset
}

type sysdumpConfig struct {
	kubeConfig              string
	context                 string
	includePolicies         bool
	includePolicyReports    bool
	includePolicyExceptions bool
}

func Command() *cobra.Command {
	sysdumpConfiguration := &sysdumpConfig{}
	cmd := &cobra.Command{
		Use:   "sysdump",
		Short: "Collect and package information for troubleshooting",
		RunE: func(cmd *cobra.Command, args []string) error {
			clients, err := initClients(sysdumpConfiguration.kubeConfig, sysdumpConfiguration.context)
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sysdumpConfiguration.kubeConfig, "kubeconfig", "", "path to kubeconfig file with authorization and master location information")
	cmd.Flags().StringVar(&sysdumpConfiguration.context, "context", "", "The name of the kubeconfig context to use")
	return cmd
}

func initClients(kubeConfig string, context string) (*client, error) {
	var clients client
	clientConfig, err := config.CreateClientConfigWithContext(kubeConfig, context)
	if err != nil {
		return nil, fmt.Errorf("failed to create client config: %v", err)
	}
	clients.kubernetesClientSet, err = kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %v", err)
	}
	clients.kyvernoClientSet, err = kyverno.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kyverno clientset: %v", err)
	}
	return &clients, nil
}
