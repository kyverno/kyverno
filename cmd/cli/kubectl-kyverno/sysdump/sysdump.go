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
	k8sv1 "k8s.io/api/core/v1"
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

type sysdumpConfig struct {
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

			if sysdumpConfiguration.includePolicies {
				err := exportClusterPoliciesInfo(clients, dir)
				if err != nil {
					return err
				}
			}

			namespaceList, err := getNamespaceList(clients)
			if err != nil {
				return err
			}

			if sysdumpConfiguration.includePolicyReports {
				err := exportPolicyReports(clients, namespaceList, dir)
				if err != nil {
					return err
				}
			}

			if sysdumpConfiguration.includePolicyExceptions {
				err := exportPolicyExceptions(clients, namespaceList, dir)
				if err != nil {
					return err
				}
			}

			if err := createArchive(dir, homeDir); err != nil {
				return err
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&sysdumpConfiguration.includePolicies, "include-policies", false, "If set to true, will export clusterpolicies to sysdump archive")
	cmd.Flags().BoolVar(&sysdumpConfiguration.includePolicyReports, "include-policy-reports", false, "If set to true, will export policy reports to sysdump archive")
	cmd.Flags().BoolVar(&sysdumpConfiguration.includePolicyExceptions, "include-policy-exceptions", false, "If set to true, will export policy exceptions to sysdump archive")
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

func getNamespaceList(clients *client) (*k8sv1.NamespaceList, error) {
	namespaceList, err := clients.kubernetesClientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get NamespaceList: %v", err)
	}
	return namespaceList, nil
}

// Create archive at home directory
func createArchive(source, archiveDestination string) error {
	sysdumpFile := archiveDestination + "/" + "kyverno-sysdump-" + strings.Replace(time.Now().Format(time.UnixDate), ":", "_", -1) + ".zip"
	if err := archiver.Archive([]string{source}, sysdumpFile); err != nil {
		return fmt.Errorf("failed to create sysdump zip file: %v", err)
	}
	fmt.Printf("Sysdump zip file created at %s\n", archiveDestination)
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

func exportClusterPoliciesInfo(clients *client, destination string) error {
	destination = filepath.Join(destination, "clusterpolicies")
	err := os.Mkdir(destination, 0700)
	if err != nil {
		return fmt.Errorf("failed to create clusterpolicies directory for sysdump: %v", err)
	}

	clusterPolicyList, err := clients.kyvernoClientSet.KyvernoV1().ClusterPolicies().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ClusterPolicyList: %v", err)
	}

	for _, clusterPolicy := range clusterPolicyList.Items {
		cp, err := clients.kyvernoClientSet.KyvernoV1().ClusterPolicies().Get(context.Background(), clusterPolicy.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get clusterpolicy: %v", err)
		}

		if err := writeYaml(path.Join(destination, clusterPolicy.Name+".yaml"), cp); err != nil {
			return err
		}
	}

	return nil
}

func exportPolicyReports(clients *client, namespaceList *k8sv1.NamespaceList, destination string) error {
	destination = filepath.Join(destination, "policy-reports")
	err := os.Mkdir(destination, 0700)
	if err != nil {
		return fmt.Errorf("failed to create policy-reports directory for sysdump: %v", err)
	}

	for _, namespace := range namespaceList.Items {
		nameSpaceName := namespace.Name
		policyReportList, err := clients.kyvernoClientSet.Wgpolicyk8sV1alpha2().PolicyReports(nameSpaceName).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to get PolicyReportList: %v", err)
		}
		for _, policyReport := range policyReportList.Items {
			polr, err := clients.kyvernoClientSet.Wgpolicyk8sV1alpha2().PolicyReports(nameSpaceName).Get(context.Background(), policyReport.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get policy report: %v", err)
			}

			fileName := nameSpaceName + "-polr-" + polr.Name
			if err := writeYaml(path.Join(destination, fileName+".yaml"), polr); err != nil {
				return err
			}
		}
	}

	return nil
}

func exportPolicyExceptions(clients *client, namespaceList *k8sv1.NamespaceList, destination string) error {
	destination = filepath.Join(destination, "policy-exceptions")
	err := os.Mkdir(destination, 0700)
	if err != nil {
		return fmt.Errorf("failed to create policy-exceptions directory for sysdump: %v", err)
	}

	for _, namespace := range namespaceList.Items {
		namespaceName := namespace.Name
		policyExceptionList, err := clients.kyvernoClientSet.KyvernoV2alpha1().PolicyExceptions(namespaceName).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to get PolicyExceptionList: %v", err)
		}
		for _, policyException := range policyExceptionList.Items {
			polex, err := clients.kyvernoClientSet.KyvernoV2alpha1().PolicyExceptions(namespaceName).Get(context.Background(), policyException.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get policy exception: %v", err)
			}

			fileName := namespaceName + "-polex-" + polex.Name
			if err := writeYaml(path.Join(destination, fileName+".yaml"), polex); err != nil {
				return err
			}
		}
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
		return fmt.Errorf("failed to print object to YAML: %v", err)
	}
	return writeToFile(p, b.String())
}
