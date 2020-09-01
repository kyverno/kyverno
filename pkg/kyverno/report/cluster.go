package report

import (
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	"sync"
)

func ClusterCommand() *cobra.Command {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	var mode string
	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "generate report",
		Example: fmt.Sprintf("To apply on a resource:\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2\n\nTo apply on a cluster\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster"),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			restConfig, err := kubernetesConfig.ToRESTConfig()
			if err != nil {
				os.Exit(1)
			}
			var wg sync.WaitGroup
			wg.Add(1)
			if mode == "cli" {
				go backgroundScan("", "Cluster", &wg, restConfig)
				wg.Wait()
				return nil
			}
			go configmapScan("", "Cluster", &wg, restConfig)
			wg.Wait()
			return nil
		},
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", "", "mode")
	return cmd
}
