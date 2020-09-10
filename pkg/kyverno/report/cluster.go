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
	var mode,policy string
	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "generate report",
		Example: fmt.Sprintf("To create a cluster report from background scan:\nkyverno report cluster --namespace=defaults \n kyverno report cluster"),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			os.Setenv("POLICY-TYPE", "POLICYREPORT")
			restConfig, err := kubernetesConfig.ToRESTConfig()
			if err != nil {
				os.Exit(1)
			}
			var wg sync.WaitGroup
			wg.Add(1)
			if mode == "cli" {
				go backgroundScan("", "Cluster",policy, &wg, restConfig)
				wg.Wait()
				return nil
			}
			go configmapScan("", "Cluster", &wg, restConfig)
			wg.Wait()
			return nil
		},
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", "cli", "mode of cli")
	cmd.Flags().StringVarP(&policy, "policy", "p", "", "define specific policy")

	return cmd
}
