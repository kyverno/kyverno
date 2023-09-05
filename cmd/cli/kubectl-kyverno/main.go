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
	if err := run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func run() error {
	cmd := commands.RootCommand()
	if err := configureLogs(cmd); err != nil {
		return fmt.Errorf("Failed to setup logging (%w)", err)
	}
	if err := cmd.Execute(); err != nil {
		return fmt.Errorf("Failed to execute command (%w)", err)
	}
	return nil
}

func configureLogs(cli *cobra.Command) error {
	logging.InitFlags(nil)
	if err := logging.Setup(logging.TextFormat, 0); err != nil {
		return err
	}
	cli.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	return nil
}
