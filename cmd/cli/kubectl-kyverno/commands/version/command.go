package version

import (
	"encoding/json"
	"fmt"
	"log"
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

var outputFormats = command.OutputFormats[func(any) ([]byte, error)]{
	{Name: "json", Render: func(value any) ([]byte, error) { return json.MarshalIndent(value, "", "  ") }},
	{Name: "yaml", Render: yaml.Marshal},
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

			marshal, ok := outputFormats.Lookup(output)
			if !ok {
				return fmt.Errorf("invalid output format: %s (supported formats: %s)", output, strings.Join(outputFormats.Names(""), ", "))
			}
			data, err := marshal(info)
			if err != nil {
				return fmt.Errorf("failed to marshal version to %s: %w", output, err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", fmt.Sprintf("Output format (%s)", strings.Join(outputFormats.Names(""), ", ")))
	if err := cmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormats.Names(toComplete), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		log.Println("WARNING", err)
	}
	return cmd
}
