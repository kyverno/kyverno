package sysdump

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type options struct {
	cluster                 string
	kubeconfig              string
	outputDir               string
	includePolicies         bool
	includePolicyReports    bool
	includePolicyExceptions bool
	includeMetrics          bool
	namespace               string
}

func Command() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "sysdump",
		Short: "Collects and packages Kyverno diagnostic information for support",
		Long: `sysdump collects cluster info, Kyverno logs, configuration, and configmaps.
Sensitive data such as Secrets are excluded. Use flags to include additional data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSysdump(opts)
		},
	}

	cmd.Flags().StringVar(&opts.cluster, "cluster", "", "Cluster context name (defaults to current kubeconfig context)")
	cmd.Flags().StringVar(&opts.kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringVar(&opts.outputDir, "output-dir", ".", "Directory to write the sysdump archive")
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "kyverno", "Namespace where Kyverno is installed")
	cmd.Flags().BoolVar(&opts.includePolicies, "include-policies", false, "Include Kyverno policies in the dump")
	cmd.Flags().BoolVar(&opts.includePolicyReports, "include-policy-reports", false, "Include policy reports in the dump")
	cmd.Flags().BoolVar(&opts.includePolicyExceptions, "include-policy-exceptions", false, "Include policy exceptions in the dump")
	cmd.Flags().BoolVar(&opts.includeMetrics, "include-metrics", false, "Include Kyverno metrics in the dump")

	return cmd
}

func runSysdump(opts *options) error {
	// 1. Build kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.kubeconfig != "" {
		loadingRules.ExplicitPath = opts.kubeconfig
	}
	configOverrides := &clientcmd.ConfigOverrides{}
	if opts.cluster != "" {
		configOverrides.CurrentContext = opts.cluster
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to build kube config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create kube client: %w", err)
	}

	// 2. Create output archive
	timestamp := time.Now().Format("20060102-150405")
	archiveName := fmt.Sprintf("kyverno-sysdump-%s.tar.gz", timestamp)
	archivePath := filepath.Join(opts.outputDir, archiveName)

	f, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	ctx := context.Background()

	// 3. Collect cluster nodes info
	fmt.Println("Collecting node info...")
	if err := collectNodes(ctx, clientset, tw); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to collect nodes: %v\n", err)
	}

	// 4. Collect Kyverno configmaps
	fmt.Println("Collecting Kyverno configmaps...")
	if err := collectConfigMaps(ctx, clientset, tw, opts.namespace); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to collect configmaps: %v\n", err)
	}

	// 5. Collect Kyverno pod logs
	fmt.Println("Collecting Kyverno logs...")
	if err := collectLogs(ctx, clientset, tw, opts.namespace); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to collect logs: %v\n", err)
	}

	// 6. Optional: policies, reports, exceptions, metrics
	if opts.includePolicies {
		fmt.Println("Collecting policies... (requires Kyverno CRD client)")
		// Use dynamic client to list ClusterPolicy + Policy CRDs
	}

	fmt.Printf("\nSysdump written to: %s\n", archivePath)
	return nil
}

func collectNodes(ctx context.Context, client kubernetes.Interface, tw *tar.Writer) error {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return err
	}
	return writeToArchive(tw, "nodes.json", string(data))
}

func collectConfigMaps(ctx context.Context, client kubernetes.Interface, tw *tar.Writer, ns string) error {
	cms, err := client.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cms, "", "  ")
	if err != nil {
		return err
	}
	return writeToArchive(tw, "configmaps.json", string(data))
}

func collectLogs(ctx context.Context, client kubernetes.Interface, tw *tar.Writer, ns string) error {
	pods, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/part-of=kyverno",
	})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		req := client.CoreV1().Pods(ns).GetLogs(pod.Name, &corev1.PodLogOptions{})
		logs, err := req.DoRaw(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping logs for %s: %v\n", pod.Name, err)
			continue
		}
		_ = writeToArchive(tw, fmt.Sprintf("logs/%s.log", pod.Name), string(logs))
	}
	return nil
}

func writeToArchive(tw *tar.Writer, name, content string) error {
	data := []byte(content)
	header := &tar.Header{
		Name:    name,
		Mode:    0644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
