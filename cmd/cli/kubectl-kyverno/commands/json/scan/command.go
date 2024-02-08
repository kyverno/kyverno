package scan

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:          "scan",
		Short:        command.FormatDescription(true, websiteUrl, true, description...),
		Long:         command.FormatDescription(false, websiteUrl, true, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         options.run,
	}
	cmd.Flags().StringVar(&options.payload, "payload", "", "Path to payload (json or yaml file)")
	cmd.Flags().StringSliceVar(&options.preprocessors, "pre-process", nil, "JMESPath expression used to pre process payload")
	cmd.Flags().StringSliceVar(&options.policies, "policy", nil, "Path to kyverno-json policies")
	cmd.Flags().StringSliceVar(&options.selectors, "labels", nil, "Labels selectors for policies")
	cmd.Flags().StringVar(&options.output, "output", "text", "Output format (text or json)")
	return cmd
}
