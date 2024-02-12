package sysdump

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"sync"

	kyverno "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/spf13/cobra"
	api "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

type client struct {
	kubernetesClientSet kubernetes.Interface
	kyvernoClientSet    kyverno.Interface
	apiClientSet        apiext.Interface
}

type sysdumpConfig struct {
	kubeConfig              string
	context                 string
	includePolicies         bool
	includePolicyReports    bool
	includePolicyExceptions bool
}

func Command() *cobra.Command {
	sysdumpConfiguration := &sysdumpConfig{}
	cmd := &cobra.Command{
		Use:   "sysdump",
		Short: "Collect and package information for troubleshooting",
		RunE: func(cmd *cobra.Command, args []string) error {
			clients, err := initClients(sysdumpConfiguration.kubeConfig, sysdumpConfiguration.context)
			if err != nil {
				return err
			}

			dirPath, err := os.MkdirTemp("", "kyverno-sysdump")
			if err != nil {
				return err
			}

			var wg sync.WaitGroup
			fetchDefaultClusterInformation(&wg, clients, dirPath)
			wg.Wait()

			return nil
		},
	}
	cmd.Flags().StringVar(&sysdumpConfiguration.kubeConfig, "kubeconfig", "", "path to kubeconfig file with authorization and master location information")
	cmd.Flags().StringVar(&sysdumpConfiguration.context, "context", "", "The name of the kubeconfig context to use")
	return cmd
}

func initClients(kubeConfig string, context string) (*client, error) {
	_ = api.AddToScheme(scheme.Scheme)
	var clients client
	clientConfig, err := config.CreateClientConfigWithContext(kubeConfig, context)
	if err != nil {
		return nil, fmt.Errorf("failed to create client config: %v", err)
	}
	clients.kubernetesClientSet, err = kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %v", err)
	}
	clients.kyvernoClientSet, err = kyverno.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kyverno clientset: %v", err)
	}
	clients.apiClientSet, err = apiext.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create api extensions clientset: %v", err)
	}
	return &clients, nil
}

// TODO: Handle all err
func fetchDefaultClusterInformation(wg *sync.WaitGroup, clients *client, dirPath string) {
	// K8s Server Version
	wg.Add(1)
	go func() {
		defer wg.Done()
		version, err := clients.kubernetesClientSet.Discovery().ServerVersion()
		if err != nil {
			return
		}
		if err := writeToFile(path.Join(dirPath, "version.txt"), version.String()); err != nil {
			return
		}
	}()

	// Nodes
	wg.Add(1)
	go func() {
		defer wg.Done()
		nodes, err := clients.kubernetesClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return
		}
		if err := writeYaml(path.Join(dirPath, "nodes-info.yaml"), nodes); err != nil {
			fmt.Println("writeYaml error:", err)
			return
		}
	}()

	// CRDS
	wg.Add(1)
	go func() {
		defer wg.Done()
		crds, err := clients.apiClientSet.ApiextensionsV1().CustomResourceDefinitions().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return
		}
		if err := writeYaml(path.Join(dirPath, "crds.yaml"), crds); err != nil {
			fmt.Println("writeYaml error:", err)
			return
		}
	}()
}

func writeToFile(path, data string) error {
	return os.WriteFile(path, []byte(data), 0600)
}

func writeYaml(path string, obj runtime.Object) error {
	printer := printers.NewTypeSetter(scheme.Scheme).ToPrinter(&printers.YAMLPrinter{})
	var b bytes.Buffer
	if err := printer.PrintObj(obj, &b); err != nil {
		return err
	}
	return writeToFile(path, b.String())
}
