package version

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

type versionInfo struct {
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

			info := versionInfo{
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
	if err := cmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		formats := make([]string, 0, 2)
		for _, format := range []string{"json", "yaml"} {
			if strings.HasPrefix(format, toComplete) {
				formats = append(formats, format)
			}
		}
		return formats, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(fmt.Errorf("failed to register output completion: %w", err))
	}
	return cmd
}
