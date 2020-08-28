package report

import (
	"fmt"

	"github.com/spf13/cobra"
)

type resultCounts struct {
	pass  int
	fail  int
	warn  int
	error int
	skip  int
}

func Command() *cobra.Command {
	var cmd *cobra.Command
	var kubeconfig string

	cmd = &cobra.Command{
		Use:     "report",
		Short:   "generate report",
		Example: fmt.Sprintf("To apply on a resource:\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2\n\nTo apply on a cluster\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster"),
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			cmd.Help()
			return err
		},
	}
	cmd.AddCommand(HelmCommand())
	cmd.AddCommand(NamespaceCommand())
	cmd.AddCommand(ClusterCommand())
	cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "kubeconfig")
	cmd.Flags().BoolP("configmap", "c", false, "kubeconfig")
	return cmd
}
