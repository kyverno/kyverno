package kyverno

import (
	"flag"
	"os"

	"github.com/nirmata/kyverno/pkg/kyverno/validate"

	"github.com/nirmata/kyverno/pkg/kyverno/apply"

	"github.com/nirmata/kyverno/pkg/kyverno/version"

	"github.com/spf13/cobra"
)

func CLI() {
	cli := &cobra.Command{
		Use:   "kyverno",
		Short: "kyverno manages native policies of Kubernetes",
	}

	configureGlog(cli)

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

func configureGlog(cli *cobra.Command) {
	flag.Parse()
	_ = flag.Set("logtostderr", "true")

	cli.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	_ = cli.PersistentFlags().MarkHidden("alsologtostderr")
	_ = cli.PersistentFlags().MarkHidden("logtostderr")
	_ = cli.PersistentFlags().MarkHidden("log_dir")
	_ = cli.PersistentFlags().MarkHidden("log_backtrace_at")
	_ = cli.PersistentFlags().MarkHidden("stderrthreshold")
	_ = cli.PersistentFlags().MarkHidden("vmodule")
}
