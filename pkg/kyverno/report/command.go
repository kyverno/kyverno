package report

import (
	"fmt"

	"github.com/spf13/cobra"
)

type resultCounts struct {
	pass  int
	fail  int
	warn  int
	error int
	skip  int
}

func Command() *cobra.Command {
	var cmd *cobra.Command
	cmd = &cobra.Command{
		Use:     "report",
		Short:   "generate report",
		Example: fmt.Sprintf("To create a report from background scan:\nkyverno report"),
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			cmd.Help()
			return err
		},
	}
	cmd.AddCommand(AppCommand())
	cmd.AddCommand(NamespaceCommand())
	cmd.AddCommand(ClusterCommand())
	cmd.AddCommand(AllReportsCommand())
	return cmd
}
