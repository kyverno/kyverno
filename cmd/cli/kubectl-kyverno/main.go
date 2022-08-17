package main

import (
	"flag"
	"os"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apply"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/jp"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/version"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// CLI ...
func main() {
	cli := &cobra.Command{
		Use:   "kyverno",
		Short: "Kubernetes Native Policy Management",
	}

	configurelog(cli)

	commands := []*cobra.Command{
		version.Command(),
		apply.Command(),
		test.Command(),
		jp.Command(),
	}

	cli.AddCommand(commands...)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

func configurelog(cli *cobra.Command) {
	klog.InitFlags(nil)
	log.SetLogger(klogr.New())

	cli.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	_ = cli.PersistentFlags().MarkHidden("alsologtostderr")
	_ = cli.PersistentFlags().MarkHidden("logtostderr")
	_ = cli.PersistentFlags().MarkHidden("log_dir")
	_ = cli.PersistentFlags().MarkHidden("log_backtrace_at")
	_ = cli.PersistentFlags().MarkHidden("stderrthreshold")
	_ = cli.PersistentFlags().MarkHidden("vmodule")
}
