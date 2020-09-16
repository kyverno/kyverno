package report

import (
	"fmt"
	"github.com/nirmata/kyverno/pkg/common"
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
	"time"
)

func AllReportsCommand() *cobra.Command {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	var mode,namespace, policy string
	cmd := &cobra.Command{
		Use:     "all",
		Short:   "generate report for all scope",
		Example: fmt.Sprintf("To create a namespace report from background scan:\nkyverno report namespace --namespace=defaults \n kyverno report namespace"),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			os.Setenv("POLICY-TYPE", common.PolicyReport)
			logger := log.Log.WithName("Report")
			restConfig, err := kubernetesConfig.ToRESTConfig()
			if err != nil {
				logger.Error(err, "failed to create rest config of kubernetes cluster ")
				os.Exit(1)
			}
			const resyncPeriod = 1 * time.Second
			kubeClient, err := utils.NewKubeClient(restConfig)
			if err != nil {
				log.Log.Error(err, "Failed to create kubernetes client")
				os.Exit(1)
			}
			var wg sync.WaitGroup
			if mode == "cli" {
				if namespace != "" {
					wg.Add(1)
					go backgroundScan(namespace, All, policy, &wg, restConfig, logger)
				} else {
					ns, err := kubeClient.CoreV1().Namespaces().List(metav1.ListOptions{})
					if err != nil {
						os.Exit(1)
					}
					wg.Add(len(ns.Items))
					for _, n := range ns.Items {
						go backgroundScan(n.GetName(), All, policy, &wg, restConfig, logger)
					}
				}
			}else{
				wg.Add(1)
				go configmapScan(All, &wg, restConfig, logger)
			}
			wg.Wait()
			os.Exit(0)
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "define specific namespace")
	cmd.Flags().StringVarP(&policy, "policy", "p", "", "define specific policy")
	cmd.Flags().StringVarP(&mode, "mode", "m", "cli", "mode")
	return cmd
}
