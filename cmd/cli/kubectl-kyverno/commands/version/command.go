package version

import (
	"encoding/json"
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

type VersionInfo struct {
	Version     string `json:"version" yaml:"version"`
	Time        string `json:"time" yaml:"time"`
	GitCommitID string `json:"gitCommitId" yaml:"gitCommitId"`
}

func Command() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:          "version",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if output == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\n", version.Version())
				fmt.Fprintf(cmd.OutOrStdout(), "Time: %s\n", version.Time())
				fmt.Fprintf(cmd.OutOrStdout(), "Git commit ID: %s\n", version.Hash())
				return nil
			}

			info := VersionInfo{
				Version:     version.Version(),
				Time:        version.Time(),
				GitCommitID: version.Hash(),
			}

			switch output {
			case "json":
				data, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal version to json: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			case "yaml":
				data, err := yaml.Marshal(info)
				if err != nil {
					return fmt.Errorf("failed to marshal version to yaml: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			default:
				return fmt.Errorf("invalid output format: %s (supported formats: json, yaml)", output)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format (json, yaml)")
	return cmd
}
