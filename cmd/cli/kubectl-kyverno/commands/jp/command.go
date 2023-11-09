package jp

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp/function"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp/parse"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp/query"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "jp",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
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
