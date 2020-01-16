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

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

func configureGlog(cli *cobra.Command) {
	flag.Parse()
	flag.Set("logtostderr", "true")

	cli.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	cli.PersistentFlags().MarkHidden("alsologtostderr")
	cli.PersistentFlags().MarkHidden("logtostderr")
	cli.PersistentFlags().MarkHidden("log_dir")
	cli.PersistentFlags().MarkHidden("log_backtrace_at")
	cli.PersistentFlags().MarkHidden("stderrthreshold")
	cli.PersistentFlags().MarkHidden("vmodule")
}
