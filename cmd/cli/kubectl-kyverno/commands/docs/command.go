package docs

import (
	"log"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/spf13/cobra"
)

func Command(root *cobra.Command) *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:          "docs",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.validate(root); err != nil {
				return err
			}
			return options.execute(root)
		},
	}
	cmd.Flags().StringVarP(&options.path, "output", "o", ".", "Output path")
	cmd.Flags().BoolVar(&options.website, "website", false, "Website version")
	cmd.Flags().BoolVar(&options.autogenTag, "autogenTag", true, "Determines if the generated docs should contain a timestamp")
	if err := cmd.MarkFlagDirname("output"); err != nil {
		log.Println("WARNING", err)
	}
	if err := cmd.MarkFlagRequired("output"); err != nil {
		log.Println("WARNING", err)
	}
	return cmd
}
