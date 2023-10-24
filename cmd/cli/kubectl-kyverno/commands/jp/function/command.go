package function

import (
	"cmp"
	"fmt"
	"io"
	"slices"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use:          "function [function_name]...",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			printFunctions(cmd.OutOrStdout(), args...)
		},
	}
}

func printFunctions(out io.Writer, names ...string) {
	functions := jmespath.GetFunctions(config.NewDefaultConfiguration(false))
	slices.SortFunc(functions, func(a, b jmespath.FunctionEntry) int {
		return cmp.Compare(a.String(), b.String())
	})
	namesSet := sets.New(names...)
	for _, function := range functions {
		if len(namesSet) == 0 || namesSet.Has(function.Name) {
			note := function.Note
			function.Note = ""
			fmt.Fprintln(out, "Name:", function.Name)
			fmt.Fprintln(out, "  Signature:", function.String())
			if note != "" {
				fmt.Fprintln(out, "  Note:     ", note)
			}
			fmt.Fprintln(out)
		}
	}
}
