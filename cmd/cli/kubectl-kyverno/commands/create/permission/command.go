package permission

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"text/template"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/templates" // Adjust the import path as necessary
	"github.com/spf13/cobra"
)

type options struct {
	Verbs        []string
	Controllers  []string // Change from string to slice
	ApiGroup     string
	ResourceType string
}

// Command initializes the cobra command
func Command() *cobra.Command {
	var verbs []string
	var opts options
	cmd := &cobra.Command{
		Use:   "permission [resource-type]",
		Short: "Create an aggregated role for a given resource type",
		Long: `This command generates a Kubernetes ClusterRole and ClusterRoleBinding for a specified resource type.
The generated files will be saved in the user's home directory under the "aggregated-role" folder.
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

			// Handle 'all' to include all verbs
			if verbs[0] == "all" {
				verbs = []string{"create", "get", "update", "delete", "list", "watch"}
			}

			tmpl, err := template.New("aggregatedRole").Parse(templates.AggregatedRoleTemplate)
			if err != nil {
				return fmt.Errorf("failed to parse template: %w", err)
			}

			opts.ResourceType = resourceType
			opts.Verbs = verbs

			if len(opts.Controllers) == 0 {
				opts.Controllers = []string{"background-controller"} // Default controller name
			}

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("could not determine the home directory: %w", err)
			}
			dirPath := fmt.Sprintf("%s/aggregated-role", homeDir)

			if _, err := os.Stat(dirPath); os.IsNotExist(err) {
				err = os.MkdirAll(dirPath, os.ModePerm)
				if err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
			}

			filePath := fmt.Sprintf("%s/%s-permission.yaml", dirPath, opts.ResourceType)
			file, err := os.Create(filePath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			defer file.Close()

			if err := tmpl.Execute(file, opts); err != nil {
				return fmt.Errorf("failed to execute template: %w", err)
			}

			if err := ApplyManifest(filePath); err != nil {
				return err
			}

			return nil
		},
	}

	// New flag to accept multiple controllers
	cmd.Flags().StringArrayVar(&opts.Controllers, "controllers", nil, "List of controllers for the ClusterRole")

	cmd.Flags().StringVarP(&opts.ApiGroup, "api-group", "g", "", "API group for the resource (required)")
	cmd.Flags().StringArrayVar(&verbs, "verbs", nil, "List of verbs for the ClusterRole or all")

	if err := cmd.MarkFlagRequired("api-group"); err != nil {
		log.Println("WARNING", err)
	}
	if err := cmd.MarkFlagRequired("verbs"); err != nil {
		log.Println("WARNING", err)
	}

	return cmd
}

// ApplyManifest applies the Kubernetes manifest using kubectl
func ApplyManifest(filePath string) error {
	cmd := exec.Command("kubectl", "apply", "-f", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply manifest: %w, output: %s", err, output)
	}

	fmt.Printf("Command output: %s\n", output)
	return nil
}
