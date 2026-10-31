package jp

import (
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp/function"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp/parse"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp/query"
	"github.com/spf13/cobra"
)

// deprecationNotice is printed once, to stderr, before jp or any of its subcommands run.
// It deliberately doesn't use cobra's Deprecated field: that hides the command from help
// output and prints to stdout, which would both bury the command users need to debug the
// JMESPath expressions they're migrating and corrupt piped `jp query`/`jp parse` output.
const deprecationNotice = "Warning: `kyverno jp` is deprecated and will be removed in 1.21, as Kyverno migrates GlobalContextEntry projections from JMESPath to CEL; it remains available to help debug existing JMESPath expressions during the migration."

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "jp",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), deprecationNotice)
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		function.Command(),
		parse.Command(),
		query.Command(),
	)
	return cmd
}
