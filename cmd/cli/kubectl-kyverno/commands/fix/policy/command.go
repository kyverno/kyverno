package policy

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:          "policy [dir]...",
		Short:        command.FormatDescription(true, websiteUrl, true, description...),
		Long:         command.FormatDescription(false, websiteUrl, true, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.validate(args...); err != nil {
				return err
			}
			return options.execute(cmd.OutOrStdout(), args...)
		},
	}
	cmd.Flags().BoolVar(&options.save, "save", false, "Save fixed file")
	return cmd
}
