package validate

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/spf13/cobra"
)

func Command(keychain authn.Keychain) *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:          "validate [dir or file]",
		Short:        command.FormatDescription(true, websiteUrl, true, description...),
		Long:         command.FormatDescription(false, websiteUrl, true, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.validate(args); err != nil {
				return err
			}
			return options.execute(cmd.Context(), args, keychain)
		},
	}
	cmd.Flags().StringVarP(&options.imageRef, "image", "i", "", "image reference of a pushed bundle to validate")
	cmd.Flags().StringVar(&options.kyvernoTarget, "kyverno-version", "", "Kyverno version to check the bundle's declared kyvernoVersion constraint against; an unsatisfied constraint fails")
	cmd.Flags().BoolVar(&options.strict, "strict", false, "treat warnings (such as an expired exception) as failures")
	return cmd
}
