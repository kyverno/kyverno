package metricsconfig

import (
	"os"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/create/templates"
	"github.com/spf13/cobra"
)

type namespaces struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

type options struct {
	Name       string
	Namespace  string
	Namespaces namespaces
}

func Command() *cobra.Command {
	var path string
	var options options
	cmd := &cobra.Command{
		Use:     "metrics-config",
		Short:   "Create a Kyverno metrics-config file.",
		Example: "kyverno create metrics-config -i ns-included-1 -i ns-included-2 -e ns-excluded",
		RunE: func(cmd *cobra.Command, args []string) error {
			tmpl, err := template.New("metricsconfig").Funcs(sprig.HermeticTxtFuncMap()).Parse(templates.MetricsConfigTemplate)
			if err != nil {
				return err
			}
			output := os.Stdout
			if path != "" {
				file, err := os.Create(path)
				if err != nil {
					return err
				}
				defer file.Close()
				output = file
			}
			return tmpl.Execute(output, options)
		},
	}
	cmd.Flags().StringVarP(&path, "output", "o", "", "Output path (uses standard console output if not set)")
	cmd.Flags().StringVarP(&options.Name, "name", "n", "kyverno-metrics", "Name")
	cmd.Flags().StringVar(&options.Namespace, "namespace", "kyverno", "Namespace")
	cmd.Flags().StringSliceVarP(&options.Namespaces.Include, "include", "i", []string{}, "Included namespaces")
	cmd.Flags().StringSliceVarP(&options.Namespaces.Exclude, "exclude", "e", []string{}, "Excluded namespaces")
	return cmd
}
