package inspect

import (
	"log"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/spf13/cobra"
)

func Command(keychain authn.Keychain) *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:          "inspect",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.validate(); err != nil {
				return err
			}
			return options.execute(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), keychain)
		},
	}
	cmd.Flags().StringVarP(&options.imageRef, "image", "i", "", "image reference (tag or digest) of a pushed bundle to inspect")
	cmd.Flags().StringVarP(&options.output, "output", "o", "", "output format (json, yaml); omit for human-readable text")
	cmd.Flags().StringVar(&options.kyvernoTarget, "kyverno-version", "", "Kyverno version to report compatibility against instead of this CLI's version")
	if err := cmd.MarkFlagRequired("image"); err != nil {
		log.Println("WARNING", err)
	}
	return cmd
}
