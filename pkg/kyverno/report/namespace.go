package report

import (
	"github.com/spf13/cobra"
	"sync"
	"time"
)

func NamespaceCommand() *cobra.Command {

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
			ns, err := dClient.ListResource("", "Namespace", "", &kyvernov1.LabelSelector{})
			if err != nil {
				os.Exit(1)
			}
			var wg sync.WaitGroup
			wg.Add(len(ns.Items))
			for _, n := range ns.Items {
				go createEngineRespone(n.GetName(), &wg, restConfig)
				wg.Wait()
			}
		},
	}
}
