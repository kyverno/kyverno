package report

import (
	"fmt"
	"github.com/nirmata/kyverno/pkg/common"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
)

func ClusterCommand() *cobra.Command {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	var mode, policy string
	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "generate report",
		Example: fmt.Sprintf("To create a cluster report from background scan:\nkyverno report cluster --namespace=defaults \n kyverno report cluster"),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			os.Setenv("POLICY-TYPE", common.PolicyReport)
			logger := log.Log.WithName("Report")
			restConfig, err := kubernetesConfig.ToRESTConfig()
			if err != nil {
				logger.Error(err, "failed to create rest config of kubernetes cluster ")
				os.Exit(1)
			}
			var wg sync.WaitGroup
			wg.Add(1)
			if mode == "cli" {
				go backgroundScan("", Cluster, policy, &wg, restConfig, logger)
				wg.Wait()
				os.Exit(0)
			}
			go configmapScan(Cluster, &wg, restConfig, logger)
			wg.Wait()
			os.Exit(0)
			return nil
		},
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", "cli", "mode of cli")
	cmd.Flags().StringVarP(&policy, "policy", "p", "", "define specific policy")

	return cmd
}
