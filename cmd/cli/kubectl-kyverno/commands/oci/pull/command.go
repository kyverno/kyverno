package pull

import (
	"log"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/spf13/cobra"
)

func Command(keychain authn.Keychain) *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:          "pull [dir]",
		Short:        command.FormatDescription(true, websiteUrl, true, description...),
		Long:         command.FormatDescription(false, websiteUrl, true, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if err := options.validate(dir); err != nil {
				return err
			}
			return options.execute(cmd.Context(), dir, keychain)
		},
	}
	cmd.Flags().StringVarP(&options.imageRef, "image", "i", "", "image reference to push to or pull from")
	if err := cmd.MarkFlagRequired("image"); err != nil {
		log.Println("WARNING", err)
	}
	return cmd
}
