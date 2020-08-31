package report

import (
	"fmt"
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubeinformers "k8s.io/client-go/informers"
	"os"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
	"time"
)

func HelmCommand() *cobra.Command {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	var mode string
	cmd := &cobra.Command{
		Use:     "helm",
		Short:   "generate report",
		Example: fmt.Sprintf("To apply on a resource:\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2\n\nTo apply on a cluster\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster"),
		RunE: func(cmd *cobra.Command, args []string) (err error) {

			restConfig, err := kubernetesConfig.ToRESTConfig()
			if err != nil {
				os.Exit(1)
			}
			const resyncPeriod = 15 * time.Minute
			kubeClient, err := utils.NewKubeClient(restConfig)
			if err != nil {
				log.Log.Error(err, "Failed to create kubernetes client")
				os.Exit(1)
			}

			kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
			if mode == "cli" {
				ns, err := kubeInformer.Core().V1().Namespaces().Lister().List(labels.Everything())
				if err != nil {
					os.Exit(1)
				}
				var wg sync.WaitGroup
				wg.Add(len(ns))
				for _, n := range ns {
					go configmapScan(n.GetName(), "Helm", &wg, restConfig)
				}
				wg.Wait()
			} else {
				var wg sync.WaitGroup
				wg.Add(1)
				go backgroundScan("", "Helm", &wg, restConfig)
				wg.Wait()
				return nil
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", "", "mode")
	return cmd
}
