package report

import (
	"fmt"
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"os"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
	"time"
)

func NamespaceCommand() *cobra.Command {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	var mode, namespace, policy string
	cmd := &cobra.Command{
		Use:     "namespace",
		Short:   "generate report",
		Example: fmt.Sprintf("To create a namespace report from background scan:\nkyverno report namespace --namespace=defaults \n kyverno report namespace"),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			os.Setenv("POLICY-TYPE", "POLICYREPORT")
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
			kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
			np := kubeInformer.Core().V1().Namespaces()

			go np.Informer().Run(stopCh)
			nSynced := np.Informer().HasSynced
			nLister := np.Lister()
			if !cache.WaitForCacheSync(stopCh, nSynced) {
				log.Log.Error(err, "Failed to create kubernetes client")
				os.Exit(1)
			}
			var wg sync.WaitGroup
			if mode == "cli" {
				if namespace != "" {
					wg.Add(1)
					go backgroundScan(namespace, "Namespace", policy, &wg, restConfig, logger)
				} else {
					ns, err := nLister.List(labels.Everything())
					if err != nil {
						os.Exit(1)
					}
					wg.Add(len(ns))
					for _, n := range ns {
						go backgroundScan(n.GetName(), "Namespace", policy, &wg, restConfig, logger)
					}
				}
			} else {
				wg.Add(1)
				go configmapScan("", "Namespace", &wg, restConfig, logger)
			}
			wg.Wait()
			<-stopCh
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "define specific namespace")
	cmd.Flags().StringVarP(&policy, "policy", "p", "", "define specific policy")
	cmd.Flags().StringVarP(&mode, "mode", "m", "cli", "mode")
	return cmd
}
