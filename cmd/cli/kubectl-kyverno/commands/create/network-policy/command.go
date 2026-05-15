package networkpolicy

import (
	"fmt"
	"os"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/spf13/cobra"
)

const netPolTemplate = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: %s
  namespace: %s
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: admission-controller
  policyTypes:
  - Ingress
  ingress:
  - from:
    - ipBlock:
        cidr: "<API_SERVER_CIDR>"
    ports:
    - protocol: TCP
      port: 9443
`

func Command() *cobra.Command {
	var path string
	var namespace string
	var name string

	cmd := &cobra.Command{
		Use:          "network-policy",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			output := cmd.OutOrStdout()
			if path != "" {
				file, err := os.Create(path)
				if err != nil {
					return err
				}
				defer file.Close()
				output = file
			}
			_, err := fmt.Fprintf(output, netPolTemplate, name, namespace)
			return err
		},
	}

	cmd.Flags().StringVarP(&path, "output", "o", "", "Output path (uses standard console output if not set)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "kyverno", "namespace for the network policy")
	cmd.Flags().StringVar(&name, "name", "kyverno-admission-controller", "name of the network policy resource")

	return cmd
}
