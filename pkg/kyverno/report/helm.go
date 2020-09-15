package report

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sync"
	"time"

	"github.com/nirmata/kyverno/pkg/common"
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

func HelmCommand() *cobra.Command {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	var mode, policy, namespace string
	cmd := &cobra.Command{
		Use:     "helm",
		Short:   "generate report",
		Example: fmt.Sprintf("To create a helm report from background scan:\nkyverno report helm --namespace=defaults \n kyverno report helm"),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			os.Setenv("POLICY-TYPE", common.PolicyReport)
			logger := log.Log.WithName("Report")
			restConfig, err := kubernetesConfig.ToRESTConfig()
			if err != nil {
				logger.Error(err, "failed to create rest config of kubernetes cluster ")
				os.Exit(1)
			}
			const resyncPeriod = 15 * time.Minute
			kubeClient, err := utils.NewKubeClient(restConfig)
			if err != nil {
				logger.Error(err, "Failed to create kubernetes client")
				os.Exit(1)
			}

			var wg sync.WaitGroup
			if mode == "cli" {
				if namespace != "" {
					wg.Add(1)
					go backgroundScan(namespace, Helm, policy, &wg, restConfig, logger)
				} else {
					ns, err := kubeClient.CoreV1().Namespaces().List(metav1.ListOptions{})
					if err != nil {
						logger.Error(err, "Failed to list all namespaces")
						os.Exit(1)
					}
					wg.Add(len(ns.Items))
					for _, n := range ns.Items {
						go backgroundScan(n.GetName(), Helm, policy, &wg, restConfig, logger)
					}
				}
			} else {
				wg.Add(1)
				go configmapScan(Helm, &wg, restConfig, logger)
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
