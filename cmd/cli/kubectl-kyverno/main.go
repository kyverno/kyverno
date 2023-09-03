package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/spf13/cobra"
)

func main() {
	cmd := commands.RootCommand()
	configureLogs(cmd)
	if err := cmd.Execute(); err != nil {
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
