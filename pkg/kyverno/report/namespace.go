package report

import (
	"fmt"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
	"time"
)

func NamespaceCommand() *cobra.Command {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	var mode string
	cmd := &cobra.Command{
		Use:     "namespace",
		Short:   "generate report",
		Example: fmt.Sprintf("To apply on a resource:\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2\n\nTo apply on a cluster\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster"),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			restConfig, err := kubernetesConfig.ToRESTConfig()
			if err != nil {
				os.Exit(1)
			}
			dClient, err := client.NewClient(restConfig, 5*time.Minute, make(chan struct{}), log.Log)
			if err != nil {
				os.Exit(1)
			}
			ns, err := dClient.ListResource("", "Namespace", "", &metav1.LabelSelector{})
			if err != nil {
				os.Exit(1)
			}
			var wg sync.WaitGroup
			wg.Add(len(ns.Items))
			for _, n := range ns.Items {
				if mode == "cli" {
					go createEngineRespone(n.GetName(), mode, &wg, restConfig)
					wg.Wait()
				}
				go backgroundScan(n.GetName(), "HELM", &wg, restConfig)
				wg.Wait()
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", "", "mode")
	return cmd
}
