package report

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/constant"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

func NamespaceCommand() *cobra.Command {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	var mode, namespace, policy string
	cmd := &cobra.Command{
		Use:     "namespace",
		Short:   "generate report for scope namespace",
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

			var stopCh <-chan struct{}
			var wg sync.WaitGroup
			if mode == "cli" {
				if namespace != "" {
					wg.Add(1)
					go backgroundScan(namespace, constant.Namespace, policy, &wg, restConfig, logger)
				} else {
					ns, err := kubeClient.CoreV1().Namespaces().List(metav1.ListOptions{})
					if err != nil {
						os.Exit(1)
					}
					wg.Add(len(ns.Items))
					for _, n := range ns.Items {
						go backgroundScan(n.GetName(), constant.Namespace, policy, &wg, restConfig, logger)
					}
				}
			} else {
				wg.Add(1)
				go configmapScan(constant.Namespace, &wg, restConfig, logger)
			}
			wg.Wait()
			os.Exit(0)
			<-stopCh
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "define specific namespace")
	cmd.Flags().StringVarP(&policy, "policy", "p", "", "define specific policy")
	cmd.Flags().StringVarP(&mode, "mode", "m", "cli", "mode")
	return cmd
}
