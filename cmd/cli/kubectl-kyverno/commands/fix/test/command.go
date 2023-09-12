package test

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:     "test [folder]...",
		Short:   command.FormatDescription(true, websiteUrl, true, description...),
		Long:    command.FormatDescription(false, websiteUrl, true, description...),
		Example: command.FormatExamples(examples...),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.validate(); err != nil {
				return err
			}
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			return options.execute(args...)
		},
	}
	cmd.Flags().StringVarP(&options.fileName, "file-name", "f", "kyverno-test.yaml", "Test filename")
	cmd.Flags().BoolVar(&options.save, "save", false, "Save fixed file")
	cmd.Flags().BoolVar(&options.compress, "compress", false, "Compress test results")
	return cmd
}
