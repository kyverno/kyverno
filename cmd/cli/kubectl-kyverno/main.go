package main

import (
	"flag"
	"os"
	"strconv"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apply"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/jp"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/oci"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/version"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/spf13/cobra"
)

const EnableExperimentalEnv = "KYVERNO_EXPERIMENTAL"

func main() {
	cli := &cobra.Command{
		Use:   "kyverno",
		Long:  "To enable experimental commands, KYVERNO_EXPERIMENTAL should be configured with true or 1.",
		Short: "Kubernetes Native Policy Management",
	}
	configureLogs(cli)
	registerCommands(cli)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

func configureLogs(cli *cobra.Command) {
	logging.InitFlags(nil)
	cli.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	_ = cli.PersistentFlags().MarkHidden("alsologtostderr")
	_ = cli.PersistentFlags().MarkHidden("logtostderr")
	_ = cli.PersistentFlags().MarkHidden("log_dir")
	_ = cli.PersistentFlags().MarkHidden("log_backtrace_at")
	_ = cli.PersistentFlags().MarkHidden("stderrthreshold")
	_ = cli.PersistentFlags().MarkHidden("vmodule")
}

func enableExperimental() bool {
	if b, err := strconv.ParseBool(os.Getenv(EnableExperimentalEnv)); err == nil {
		return b
	}
	return false
}

func registerCommands(cli *cobra.Command) {
	cli.AddCommand(version.Command(), apply.Command(), test.Command(), jp.Command())
	if enableExperimental() {
		cli.AddCommand(oci.Command())
	}
}
