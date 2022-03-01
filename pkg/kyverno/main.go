package kyverno

import (
	"flag"
	"os"

	"github.com/kyverno/kyverno/pkg/kyverno/apply"
	"github.com/kyverno/kyverno/pkg/kyverno/jp"
	"github.com/kyverno/kyverno/pkg/kyverno/test"
	"github.com/kyverno/kyverno/pkg/kyverno/validate"
	"github.com/kyverno/kyverno/pkg/kyverno/version"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// CLI ...
func CLI() {
	cli := &cobra.Command{
		Use:   "kyverno",
		Short: "Kubernetes Native Policy Management",
	}

	configurelog(cli)

	commands := []*cobra.Command{
		version.Command(),
		apply.Command(),
		validate.Command(),
		test.Command(),
		jp.Command(),
	}

	cli.AddCommand(commands...)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

func configurelog(cli *cobra.Command) {
	if flag.CommandLine.Lookup("log_dir") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}
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
