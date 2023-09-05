package jp

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp/function"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp/parse"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp/query"
	cobrautils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/cobra"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "jp",
		Short:   cobrautils.FormatDescription(true, websiteUrl, false, description...),
		Long:    cobrautils.FormatDescription(false, websiteUrl, false, description...),
		Example: cobrautils.FormatExamples(examples...),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(query.Command())
	cmd.AddCommand(function.Command())
	cmd.AddCommand(parse.Command())
	return cmd
}
