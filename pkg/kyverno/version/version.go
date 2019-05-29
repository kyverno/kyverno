package version

import (
	"fmt"
	"io"

	"github.com/nirmata/kyverno/pkg/version"
	"github.com/spf13/cobra"
)

// NewCmdVersion is a command to display the build version
func NewCmdVersion(cmdOut io.Writer) *cobra.Command {

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "",
		Run: func(cmd *cobra.Command, args []string) {
			showVersion()
		},
	}

	return versionCmd
}

func showVersion() {
	fmt.Printf("Version: %s\n", version.BuildVersion)
	fmt.Printf("Time: %s\n", version.BuildTime)
	fmt.Printf("Git commit ID: %s\n", version.BuildHash)
}
