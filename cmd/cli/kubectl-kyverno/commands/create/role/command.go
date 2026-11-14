package role

import (
	"fmt"
	"log"
	"os"
	"strings"
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

// Command returns a cobra.Command for creating an aggregated ClusterRole.
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

			// Validate and trim controllers
			if len(opts.Controllers) == 0 {
				return fmt.Errorf("invalid controller provided")
			}
			var controllers []string
			for _, c := range opts.Controllers {
				trimmed := strings.TrimSpace(c)
				if trimmed == "" {
					return fmt.Errorf("invalid controller provided")
				}
				controllers = append(controllers, trimmed)
			}
			opts.Controllers = controllers

			// Validate and trim resources
			var resources []string
			for _, r := range opts.ResourceTypes {
				trimmed := strings.TrimSpace(r)
				if trimmed == "" {
					return fmt.Errorf("invalid resource provided")
				}
				resources = append(resources, trimmed)
			}
			opts.ResourceTypes = resources

			// Validate and trim verbs
			var trimmedVerbs []string
			for _, v := range verbs {
				trimmed := strings.TrimSpace(v)
				if trimmed == "" {
					return fmt.Errorf("invalid verb provided")
				}
				trimmedVerbs = append(trimmedVerbs, trimmed)
			}

			// Handle 'all' verb while preserving explicit verbs
			hasAll := false
			for _, v := range trimmedVerbs {
				if v == "all" {
					hasAll = true
					break
				}
			}
			if hasAll {
				standardVerbs := []string{"create", "get", "update", "delete", "list", "watch"}
				seen := make(map[string]bool)
				var expanded []string
				for _, sv := range standardVerbs {
					seen[sv] = true
					expanded = append(expanded, sv)
				}
				for _, v := range trimmedVerbs {
					if v != "all" && !seen[v] {
						seen[v] = true
						expanded = append(expanded, v)
					}
				}
				opts.Verbs = expanded
			} else {
				opts.Verbs = trimmedVerbs
			}

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
	cmd.Flags().StringSliceVar(&opts.Controllers, "controllers", []string{"background-controller"}, "List of controllers for the ClusterRole (default = background-controller)")
	cmd.Flags().StringVarP(&path, "output", "o", "", "Output file path (prints to console if not set)")
	cmd.Flags().StringVarP(&opts.ApiGroup, "api-groups", "g", "", "API group for the resource (required)")
	cmd.Flags().StringSliceVar(&verbs, "verbs", nil, "A comma separated list of verbs or 'all' for all verbs")
	cmd.Flags().StringSliceVar(&opts.ResourceTypes, "resources", nil, "A comma separated list of resources (required)")

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
