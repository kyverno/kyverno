package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apply"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/create"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/docs"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/fix"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/jp"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/oci"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/version"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/spf13/cobra"
)

const enableExperimentalEnv = "KYVERNO_EXPERIMENTAL"

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
	if err := logging.Setup(logging.TextFormat, 0); err != nil {
		fmt.Println("failed to setup logging", err)
		os.Exit(1)
	}
	cli.PersistentFlags().AddGoFlagSet(flag.CommandLine)
}

func enableExperimental() bool {
	if b, err := strconv.ParseBool(os.Getenv(enableExperimentalEnv)); err == nil {
		return b
	}
	return false
}

func registerCommands(cli *cobra.Command) {
	cli.AddCommand(
		apply.Command(),
		create.Command(),
		docs.Command(cli),
		jp.Command(),
		test.Command(),
		version.Command(),
	)
	if enableExperimental() {
		cli.AddCommand(
			fix.Command(),
			oci.Command(),
		)
	}
}
