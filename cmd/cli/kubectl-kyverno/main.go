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
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const EnableExperimentalEnv = "KYVERNO_EXPERIMENTAL"

func main() {
	cli := &cobra.Command{
		Use:   "kyverno",
		Long:  `To enable experimental commands, KYVERNO_EXPERIMENTAL should be configured with true or 1.`,
		Short: "Kubernetes Native Policy Management",
	}

	configurelog(cli)

	commands := []*cobra.Command{
		version.Command(),
		apply.Command(),
		test.Command(),
		jp.Command(),
	}

	if enableExperimental() {
		commands = append(commands, oci.Command())
	}

	cli.AddCommand(commands...)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

func enableExperimental() bool {
	if b, err := strconv.ParseBool(os.Getenv(EnableExperimentalEnv)); err == nil {
		return b
	}
	return false
}

func configurelog(cli *cobra.Command) {
	// clear flags initialized in static dependencies
	if flag.CommandLine.Lookup("log_dir") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}

	klog.InitFlags(nil)
	cli.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	log.SetLogger(klogr.New())

	_ = cli.PersistentFlags().MarkHidden("alsologtostderr")
	_ = cli.PersistentFlags().MarkHidden("logtostderr")
	_ = cli.PersistentFlags().MarkHidden("log_dir")
	_ = cli.PersistentFlags().MarkHidden("log_backtrace_at")
	_ = cli.PersistentFlags().MarkHidden("stderrthreshold")
	_ = cli.PersistentFlags().MarkHidden("vmodule")
}
