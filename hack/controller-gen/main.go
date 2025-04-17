package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-tools/pkg/crd"
	"sigs.k8s.io/controller-tools/pkg/genall"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

var (
	allGenerators = map[string]genall.Generator{
		"crd": crd.Generator{},
	}
	allOutputRules = map[string]genall.OutputRule{
		"dir":       genall.OutputToDirectory(""),
		"none":      genall.OutputToNothing,
		"stdout":    genall.OutputToStdout,
		"artifacts": genall.OutputArtifacts{},
	}
	optionsRegistry = &markers.Registry{}
)

func init() { //nolint:gochecknoinits
	for genName, gen := range allGenerators {
		// make the generator options marker itself
		defn := markers.Must(markers.MakeDefinition(genName, markers.DescribesPackage, gen))
		if err := optionsRegistry.Register(defn); err != nil {
			panic(err)
		}
		if helpGiver, hasHelp := gen.(genall.HasHelp); hasHelp {
			if help := helpGiver.Help(); help != nil {
				optionsRegistry.AddHelp(defn, help)
			}
		}
		// make per-generation output rule markers
		for ruleName, rule := range allOutputRules {
			ruleMarker := markers.Must(markers.MakeDefinition(fmt.Sprintf("output:%s:%s", genName, ruleName), markers.DescribesPackage, rule))
			if err := optionsRegistry.Register(ruleMarker); err != nil {
				panic(err)
			}
			if helpGiver, hasHelp := rule.(genall.HasHelp); hasHelp {
				if help := helpGiver.Help(); help != nil {
					optionsRegistry.AddHelp(ruleMarker, help)
				}
			}
		}
	}
	// make "default output" output rule markers
	for ruleName, rule := range allOutputRules {
		ruleMarker := markers.Must(markers.MakeDefinition("output:"+ruleName, markers.DescribesPackage, rule))
		if err := optionsRegistry.Register(ruleMarker); err != nil {
			panic(err)
		}
		if helpGiver, hasHelp := rule.(genall.HasHelp); hasHelp {
			if help := helpGiver.Help(); help != nil {
				optionsRegistry.AddHelp(ruleMarker, help)
			}
		}
	}
	// add in the common options markers
	if err := genall.RegisterOptionsMarkers(optionsRegistry); err != nil {
		panic(err)
	}
}

type noUsageError struct{ error }

func main() {
	cmd := &cobra.Command{
		Use: "controller-gen",
		RunE: func(c *cobra.Command, rawOpts []string) error {
			oneOf, err := markers.MakeAnyTypeDefinition("kubebuilder:oneOf", markers.DescribesType, OneOf{})
			if err != nil {
				return err
			}
			not, err := markers.MakeAnyTypeDefinition("kubebuilder:not", markers.DescribesType, Not{})
			if err != nil {
				return err
			}
			// otherwise, set up the runtime for actually running the generators
			rt, err := genall.FromOptions(optionsRegistry, rawOpts)
			if err != nil {
				return err
			}
			if err := rt.Collector.Registry.Register(oneOf); err != nil {
				return err
			}
			if err := rt.Collector.Registry.Register(not); err != nil {
				return err
			}
			if len(rt.Generators) == 0 {
				return fmt.Errorf("no generators specified")
			}
			if hadErrs := rt.Run(); hadErrs {
				// don't obscure the actual error with a bunch of usage
				return noUsageError{fmt.Errorf("not all generators ran successfully")}
			}
			return nil
		},
		SilenceUsage: true, // silence the usage, then print it out ourselves if it wasn't suppressed
	}
	if err := cmd.Execute(); err != nil {
		if _, noUsage := err.(noUsageError); !noUsage {
			// print the usage unless we suppressed it
			if err := cmd.Usage(); err != nil {
				panic(err)
			}
		}
		fmt.Fprintf(cmd.OutOrStderr(), "run `%[1]s %[2]s -w` to see all available markers, or `%[1]s %[2]s -h` for usage\n", cmd.CalledAs(), strings.Join(os.Args[1:], " "))
		os.Exit(1)
	}
}
