package role

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
	Verbs         []string
	Controllers   []string
	ApiGroup      string
	ResourceTypes []string
	Name          string
}

func Command() *cobra.Command {
	var verbs []string
	var path string
	var opts options

	cmd := &cobra.Command{
		Use:   "cluster-role [name] ",
		Short: "Create an aggregated role for given resource types",
		Long: `This command generates a Kubernetes ClusterRole for specified resource types.
The output is printed to stdout by default or saved to a specified file.
Required flags include 'api-groups', 'verbs', and 'resources'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate input arguments
			if args[0] == "" {
				return fmt.Errorf("name argument is required")
			}
			opts.Name = args[0]

			if opts.ApiGroup == "" {
				return fmt.Errorf("required flag(s) \"api-groups\" not set")
			}
			if len(opts.ResourceTypes) == 0 {
				return fmt.Errorf("required flag(s) \"resources\" not set")
			}
			if len(verbs) == 0 {
				return fmt.Errorf("required flag(s) \"verbs\" not set")
			}

			if len(opts.Controllers) == 0 || (len(opts.Controllers) == 1 && opts.Controllers[0] == "") {
				return fmt.Errorf("invalid controller provided")
			}

			// Handle 'all' verb
			if verbs[0] == "all" {
				verbs = []string{"create", "get", "update", "delete", "list", "watch"}
			}
			opts.Verbs = verbs

			// Parse the role template
			tmpl, err := template.New("aggregatedRole").Funcs(sprig.HermeticTxtFuncMap()).Parse(templates.AggregatedRoleTemplate)
			if err != nil {
				return fmt.Errorf("failed to parse template: %w", err)
			}

			// Set output: file or stdout
			output := cmd.OutOrStdout()
			if path != "" {
				file, err := os.Create(path)
				if err != nil {
					return fmt.Errorf("failed to create file: %w", err)
				}
				defer file.Close()
				output = file
			}

			// Execute template with name and options
			return tmpl.Execute(output, opts)
		},
	}

	// Define flags
	cmd.Flags().StringArrayVar(&opts.Controllers, "controllers", []string{"background-controller"}, "List of controllers for the ClusterRole (default = background-controller)")
	cmd.Flags().StringVarP(&path, "output", "o", "", "Output file path (prints to console if not set)")
	cmd.Flags().StringVarP(&opts.ApiGroup, "api-groups", "g", "", "API group for the resource (required)")
	cmd.Flags().StringArrayVar(&verbs, "verbs", nil, "A comma separated list of verbs or 'all' for all verbs")
	cmd.Flags().StringArrayVar(&opts.ResourceTypes, "resources", nil, "A comma separated list of resources (required)")

	// Mark required flags
	if err := cmd.MarkFlagRequired("api-groups"); err != nil {
		log.Println("WARNING", err)
	}
	if err := cmd.MarkFlagRequired("verbs"); err != nil {
		log.Println("WARNING", err)
	}
	if err := cmd.MarkFlagRequired("resources"); err != nil {
		log.Println("WARNING", err)
	}

	return cmd
}
