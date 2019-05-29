package version

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var (
	buildVersion = "--"
	buildHash    = "--"
	buildTime    = "--"
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
	fmt.Printf("Version: %s\n", buildVersion)
	fmt.Printf("Time: %s\n", buildTime)
	fmt.Printf("Git commit ID: %s\n", buildHash)
}
