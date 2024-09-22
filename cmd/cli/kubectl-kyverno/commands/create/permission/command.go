package permission

import (
	"fmt"
	"log"
	"os"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/templates"
	"github.com/spf13/cobra"
)

type options struct {
	Verbs        []string
	Controllers  []string
	ApiGroup     string
	ResourceType string
}

func Command() *cobra.Command {
	var verbs []string
	var path string
	var opts options
	cmd := &cobra.Command{
		Use:   "permission [resource-type]",
		Short: "Create an aggregated role for a given resource type",
		Long: `This command generates a Kubernetes ClusterRole for a specified resource type.
The output is printed to stdout by default or saved to a specified file.
Required flags include 'api-group' and 'verbs'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceType := args[0]

			if opts.ApiGroup == "" {
				return fmt.Errorf("required flag(s) \"api-group\" not set")
			}
			if resourceType == "" {
				return fmt.Errorf("the resource type argument is required")
			}
			if len(verbs) == 0 {
				return fmt.Errorf("required flag(s) \"verbs\" not set")
			}

			if verbs[0] == "all" {
				verbs = []string{"create", "get", "update", "delete", "list", "watch"}
			}

			tmpl, err := template.New("aggregatedRole").Funcs(sprig.HermeticTxtFuncMap()).Parse(templates.AggregatedRoleTemplate)
			if err != nil {
				return fmt.Errorf("failed to parse template: %w", err)
			}

			opts.ResourceType = resourceType
			opts.Verbs = verbs

			if len(opts.Controllers) == 0 {
				opts.Controllers = []string{"background-controller"} // Default controller name
			}

			// Set the output destination: stdout or file
			output := cmd.OutOrStdout()
			if path != "" {
				file, err := os.Create(path)
				if err != nil {
					return fmt.Errorf("failed to create file: %w", err)
				}
				defer file.Close()
				output = file
			}

			// Execute the template and write the output
			return tmpl.Execute(output, opts)
		},
	}

	cmd.Flags().StringArrayVar(&opts.Controllers, "controllers", nil, "List of controllers for the ClusterRole")

	cmd.Flags().StringVarP(&path, "output", "o", "", "Output file path (prints to console if not set)")

	cmd.Flags().StringVarP(&opts.ApiGroup, "api-group", "g", "", "API group for the resource (required)")
	cmd.Flags().StringArrayVar(&verbs, "verbs", nil, "List of verbs for the ClusterRole or 'all' for all verbs")

	if err := cmd.MarkFlagRequired("api-group"); err != nil {
		log.Println("WARNING", err)
	}
	if err := cmd.MarkFlagRequired("verbs"); err != nil {
		log.Println("WARNING", err)
	}

	return cmd
}
