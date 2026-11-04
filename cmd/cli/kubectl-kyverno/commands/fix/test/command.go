package test

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var options options
	var removeColor bool
	cmd := &cobra.Command{
		Use:          "test [dir]...",
		Short:        command.FormatDescription(true, websiteUrl, true, description...),
		Long:         command.FormatDescription(false, websiteUrl, true, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			color.Init(removeColor)
			if err := options.validate(args...); err != nil {
				return err
			}
			return options.execute(cmd.OutOrStdout(), args...)
		},
	}
	cmd.Flags().StringVarP(&options.fileName, "file-name", "f", "kyverno-test.yaml", "Test filename")
	cmd.Flags().BoolVar(&options.save, "save", false, "Save fixed file")
	cmd.Flags().BoolVar(&options.force, "force", false, "Force save file")
	cmd.Flags().BoolVar(&options.compress, "compress", false, "Compress test results")
	cmd.Flags().BoolVar(&removeColor, "remove-color", false, "Remove any color from output")
	return cmd
}
