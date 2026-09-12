package legacypolicies

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// maxNamesPerKind caps how many offending resource names are printed per
// kind, so a cluster with many legacy resources still produces readable
// output.
const maxNamesPerKind = 5

// legacyKind names one legacy kyverno.io policy type this command checks
// for, by its CRD name and display Kind. Everything else about the type
// (group, stored version, plural, scope) is read from the live CRD, not
// hardcoded here: the API server converts objects to the requested version
// on read, so listing at the CRD's current stored version is enough to see
// every instance regardless of the version it was written with, and a
// hardcoded version would silently miss a cluster whose CRD storage
// version has moved on.
type legacyKind struct {
	CRDName string
	Kind    string
}

var legacyKinds = []legacyKind{
	{CRDName: "clusterpolicies.kyverno.io", Kind: "ClusterPolicy"},
	{CRDName: "policies.kyverno.io", Kind: "Policy"},
	{CRDName: "cleanuppolicies.kyverno.io", Kind: "CleanupPolicy"},
	{CRDName: "clustercleanuppolicies.kyverno.io", Kind: "ClusterCleanupPolicy"},
	{CRDName: "policyexceptions.kyverno.io", Kind: "PolicyException"},
}

// kindResult holds the outcome of checking a single legacy kind.
type kindResult struct {
	Kind  string
	Count int
	Names []string
}

type options struct {
	KubeConfig string
	Context    string
}

func Command() *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:          "check-legacy-policies",
		Short:        "Detect legacy kyverno.io policy resources (internal, used by the Helm upgrade gate).",
		Hidden:       true,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientConfig, err := config.CreateClientConfigWithContext(options.KubeConfig, options.Context)
			if err != nil {
				return err
			}
			apiServerClient, err := apiextensionsclientset.NewForConfig(clientConfig)
			if err != nil {
				return err
			}
			dynamicClient, err := dynamic.NewForConfig(clientConfig)
			if err != nil {
				return err
			}
			return run(context.Background(), cmd.OutOrStdout(), apiServerClient, dynamicClient)
		},
	}
	cmd.Flags().StringVar(&options.KubeConfig, "kubeconfig", "", "path to kubeconfig file with authorization and master location information")
	cmd.Flags().StringVar(&options.Context, "context", "", "The name of the kubeconfig context to use")
	return cmd
}

// run checks every legacy kind, prints a report, and returns a non-nil
// error (without extra formatting, since the report already carries the
// detail) when any legacy resources were found.
func run(ctx context.Context, out io.Writer, apiServerClient apiextensionsclientset.Interface, dynamicClient dynamic.Interface) error {
	results, total, err := checkLegacyPolicies(ctx, apiServerClient, dynamicClient)
	if err != nil {
		return err
	}
	printResults(out, results, total)
	if total > 0 {
		return fmt.Errorf("found %d legacy policy resource(s); migrate them to policies.kyverno.io before continuing, see %s (or set upgrade.allowLegacyPolicies=true to bypass this check)", total, deprecations.MigrationGuideURL)
	}
	return nil
}

// checkLegacyPolicies lists every legacy kind and returns one kindResult
// per kind, in the declared order, plus the total count across all kinds.
func checkLegacyPolicies(ctx context.Context, apiServerClient apiextensionsclientset.Interface, dynamicClient dynamic.Interface) ([]kindResult, int, error) {
	results := make([]kindResult, 0, len(legacyKinds))
	total := 0
	for _, lk := range legacyKinds {
		result, err := countKind(ctx, apiServerClient, dynamicClient, lk)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, result)
		total += result.Count
	}
	return results, total, nil
}

// countKind counts instances of a single legacy kind. A CRD that is not
// registered on the cluster is treated as zero instances, not an error, so
// a fresh install without the legacy CRDs (or with crds.install=false)
// does not trip this check. Once the CRD is confirmed to exist, any other
// error - including a list against its stored version failing - is
// reported rather than swallowed, because at that point a false "zero"
// would let legacy resources slip past the gate undetected.
func countKind(ctx context.Context, apiServerClient apiextensionsclientset.Interface, dynamicClient dynamic.Interface, lk legacyKind) (kindResult, error) {
	crd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, lk.CRDName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return kindResult{Kind: lk.Kind}, nil
		}
		return kindResult{}, fmt.Errorf("checking CRD %s: %w", lk.CRDName, err)
	}

	var storedVersion string
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Storage {
			storedVersion = crd.Spec.Versions[i].Name
			break
		}
	}
	if storedVersion == "" {
		return kindResult{}, fmt.Errorf("CRD %s: no stored version found", lk.CRDName)
	}

	namespaced := crd.Spec.Scope == apiextensionsv1.NamespaceScoped
	gvr := schema.GroupVersionResource{Group: crd.Spec.Group, Version: storedVersion, Resource: crd.Spec.Names.Plural}
	resource := dynamicClient.Resource(gvr)

	var list *unstructured.UnstructuredList
	if namespaced {
		list, err = resource.Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	} else {
		list, err = resource.List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return kindResult{}, fmt.Errorf("listing %s/%s %s: %w", crd.Spec.Group, storedVersion, crd.Spec.Names.Plural, err)
	}

	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		name := list.Items[i].GetName()
		if namespaced {
			if ns := list.Items[i].GetNamespace(); ns != "" {
				name = ns + "/" + name
			}
		}
		names = append(names, name)
	}
	// Sort the full set before truncating, so which names are shown is
	// deterministic (alphabetically first) rather than dependent on
	// whatever order the server happened to return the list in.
	sort.Strings(names)
	if len(names) > maxNamesPerKind {
		names = names[:maxNamesPerKind]
	}
	return kindResult{Kind: lk.Kind, Count: len(list.Items), Names: names}, nil
}

func printResults(out io.Writer, results []kindResult, total int) {
	if total == 0 {
		fmt.Fprintln(out, "no legacy policy resources found")
		return
	}
	fmt.Fprintf(out, "found %d legacy policy resource(s):\n", total)
	for _, result := range results {
		if result.Count == 0 {
			continue
		}
		fmt.Fprintf(out, "  - %s: %d\n", result.Kind, result.Count)
		for _, name := range result.Names {
			fmt.Fprintf(out, "      %s\n", name)
		}
		if result.Count > len(result.Names) {
			fmt.Fprintf(out, "      ... and %d more\n", result.Count-len(result.Names))
		}
	}
	fmt.Fprintf(out, "migrate these resources to the policies.kyverno.io policy types, see %s\n", deprecations.MigrationGuideURL)
	fmt.Fprintln(out, "to bypass this check, set upgrade.allowLegacyPolicies=true")
}
