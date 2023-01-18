package function

import (
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/sets"
)

var description = []string{
	"Provides function informations",
	"For more information visit: https://kyverno.io/docs/writing-policies/jmespath/ ",
}

var examples = []string{
	"  # List functions    \n  kyverno jp function",
	"  # Get function infos\n  kyverno jp function <function name>",
}

func Command() *cobra.Command {
	return &cobra.Command{
		Use:          "function [function_name]...",
		Short:        description[0],
		Long:         strings.Join(description, "\n"),
		Example:      strings.Join(examples, "\n\n"),
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			printFunctions(args...)
		},
	}
}

func printFunctions(names ...string) {
	functions := jmespath.GetFunctions()
	slices.SortFunc(functions, func(a, b *jmespath.FunctionEntry) bool {
		return a.String() < b.String()
	})
	namesSet := sets.New(names...)
	for _, function := range functions {
		if len(namesSet) == 0 || namesSet.Has(function.Entry.Name) {
			function := *function
			note := function.Note
			function.Note = ""
			fmt.Println("Name:", function.Entry.Name)
			fmt.Println("  Signature:", function.String())
			if note != "" {
				fmt.Println("  Note:     ", note)
			}
			fmt.Println()
		}
	}
}
