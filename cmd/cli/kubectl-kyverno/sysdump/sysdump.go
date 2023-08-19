package sysdump

import (
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sysdump",
		Short: "Collect and package information for troubleshooting",

		Run: func(cmd *cobra.Command, args []string) {

		},
	}
	return cmd
}
