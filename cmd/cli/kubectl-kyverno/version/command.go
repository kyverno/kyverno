package version

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/version"
	"github.com/spf13/cobra"
)

// Command returns version command
func Command() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Shows current version of kyverno",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Version: %s\n", version.BuildVersion)
			fmt.Printf("Time: %s\n", version.BuildTime)
			fmt.Printf("Git commit ID: %s\n", version.BuildHash)
			return nil
		},
	}
}
