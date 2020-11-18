package kyverno

import (
	"flag"
	"os"

	"github.com/kyverno/kyverno/pkg/kyverno/apply"
	"github.com/kyverno/kyverno/pkg/kyverno/validate"
	"github.com/kyverno/kyverno/pkg/kyverno/version"
	"github.com/spf13/cobra"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// CLI ...
func CLI() {
	cli := &cobra.Command{
		Use:   "kyverno",
		Short: "kyverno manages native policies of Kubernetes",
	}

	configurelog(cli)

	commands := []*cobra.Command{
		version.Command(),
		apply.Command(),
		validate.Command(),
	}

	cli.AddCommand(commands...)

	cli.SilenceUsage = true

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
