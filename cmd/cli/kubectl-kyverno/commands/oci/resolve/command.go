package resolve

import (
	"log"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/spf13/cobra"
)

func Command(keychain authn.Keychain) *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:          "resolve",
		Short:        command.FormatDescription(true, websiteUrl, true, description...),
		Long:         command.FormatDescription(false, websiteUrl, true, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.validate(); err != nil {
				return err
			}
			return options.execute(cmd.Context(), cmd.OutOrStdout(), keychain)
		},
	}
	cmd.Flags().StringVarP(&options.imageRef, "image", "i", "", "image reference (tag or digest) of a pushed bundle to resolve")
	cmd.Flags().BoolVar(&options.digestOnly, "digest-only", false, "print only the digest (sha256:...) instead of the full digest-pinned reference")
	if err := cmd.MarkFlagRequired("image"); err != nil {
		log.Println("WARNING", err)
	}
	return cmd
}
