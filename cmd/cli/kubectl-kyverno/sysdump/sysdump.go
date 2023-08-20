package sysdump

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	kyverno "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoscheme "github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	"github.com/mholt/archiver"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type client struct {
	kubernetesClientSet *kubernetes.Clientset
	kyvernoClientSet    *kyverno.Clientset
}

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sysdump",
		Short: "Collect and package information for troubleshooting",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir := homedir.HomeDir()

			clients, err := initializeClients(homeDir)
			if err != nil {
				return err
			}

			dir, err := os.MkdirTemp("", "kyverno-sysdump")
			if err != nil {
				return err
			}

			if err := exportNodesInfo(clients, dir); err != nil {
				return err
			}

			if err := createArchive(dir, homeDir); err != nil {
				return err
			}

			return nil
		},
	}
	return cmd
}

func initializeClients(homeDir string) (*client, error) {
	var clients client

	kubeconfig := filepath.Join(homeDir, ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig file: %v", err)
	}

	clients.kubernetesClientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %v", err)
	}
	clients.kyvernoClientSet, err = kyverno.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kyverno clientset: %v", err)
	}
	_ = kyvernoscheme.AddToScheme(scheme.Scheme)

	return &clients, nil
}

// Create archive at home directory
func createArchive(source, archiveDestination string) error {
	sysdumpFile := archiveDestination + "/" + "kyverno-sysdump-" + strings.Replace(time.Now().Format(time.UnixDate), ":", "_", -1) + ".zip"
	if err := archiver.Archive([]string{source}, sysdumpFile); err != nil {
		return fmt.Errorf("failed to create sysdump zip file: %v", err)
	}
	fmt.Printf("Sysdump zip file created at %s", archiveDestination)
	return nil
}

func exportNodesInfo(clients *client, destination string) error {
	n, err := clients.kubernetesClientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get NodeList: %v", err)
	}

	if err := writeYaml(path.Join(destination, "nodes-info.yaml"), n); err != nil {
		return err
	}
	return nil
}

func writeToFile(p, v string) error {
	return os.WriteFile(p, []byte(v), 0600)
}

func writeYaml(p string, o runtime.Object) error {
	var j printers.YAMLPrinter
	w, err := printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(&j, nil)
	if err != nil {
		return fmt.Errorf("failed to wrap to printer: %v", err)
	}
	var b bytes.Buffer
	if err := w.PrintObj(o, &b); err != nil {
		return fmt.Errorf("failed to print %v to YAML: %v", o, err)
	}
	return writeToFile(p, b.String())
}
