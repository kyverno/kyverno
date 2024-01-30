package sysdump

import (
	kyverno "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type client struct {
	kubernetesClientSet *kubernetes.Clientset
	kyvernoClientSet    *kyverno.Clientset
}

type sysdumpConfig struct {
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
			return nil
		},
	}
	return cmd
}
