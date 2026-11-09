package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/spf13/cobra"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)

// options holds configuration flags and parameters for the migrate command.
type options struct {
	KubeConfig string
	Context    string
	Resources  []string
}

// Command returns the Cobra command for migrating CRD storage versions and existing custom resources.
func Command() *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for i, resource := range options.Resources {
				resource = strings.TrimSpace(resource)
				if resource == "" {
					return errors.New("resource cannot be empty")
				}
				options.Resources[i] = resource
			}

			clientConfig, err := config.CreateClientConfigWithContext(options.KubeConfig, options.Context)
			if err != nil {
				return err
			}
			apiServerClient, err := clientset.NewForConfig(clientConfig)
			if err != nil {
				return err
			}
			dynamicClient, err := dynamic.NewForConfig(clientConfig)
			if err != nil {
				return err
			}
			return migrateResources(cmd.Context(), options.Resources, apiServerClient, dynamicClient)
		},
	}
	cmd.Flags().StringVar(&options.KubeConfig, "kubeconfig", "", "path to kubeconfig file with authorization and master location information")
	cmd.Flags().StringVar(&options.Context, "context", "", "The name of the kubeconfig context to use")
	cmd.Flags().StringSliceVar(&options.Resources, "resource", nil, "The resource to migrate")
	if err := cmd.MarkFlagRequired("resource"); err != nil {
		cmd.PrintErrln("WARNING", err)
	}
	return cmd
}

// migrateResources iterates through the provided CRD names, looks up each CRD, and migrates its instances and status.
// Missing CRDs (NotFound errors) are logged as informational messages and skipped cleanly.
func migrateResources(ctx context.Context, resources []string, apiServerClient clientset.Interface, dynamicClient dynamic.Interface) error {
	for _, resource := range resources {
		fmt.Println("migrating resource:", resource, "...")
		crd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, resource, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				fmt.Printf("crd %s not found on cluster, skipping\n", resource)
				continue
			}
			return err
		}
		if err := migrate(ctx, crd, dynamicClient, apiServerClient); err != nil {
			return err
		}
	}
	return nil
}

// migrate coordinates the storage-version migration of a single CRD:
// 1. Identifies the active storage version from the CRD spec.
// 2. Skips legacy policy kinds whose writes are blocked in Kyverno 1.20+.
// 3. Lists and rewrites all existing instances of the resource so they are stored in the active storage version.
// 4. Retries on conflict, verifies the CRD UID/Generation/StorageVersion identity, and updates status.storedVersions.
func migrate(ctx context.Context, crd *v1.CustomResourceDefinition, dynamicClient dynamic.Interface, apiServerClient clientset.Interface) error {
	var storedVersion *v1.CustomResourceDefinitionVersion
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Storage {
			storedVersion = &crd.Spec.Versions[i]
		}
	}
	if storedVersion == nil {
		return errors.New("stored version not found")
	}

	fmt.Println("stored version:", storedVersion.Name)
	if len(crd.Status.StoredVersions) == 1 && crd.Status.StoredVersions[0] == storedVersion.Name {
		fmt.Println("stored version is already up to date, nothing to do")
		return nil
	}

	// Legacy policy kinds (ClusterPolicy, Policy, CleanupPolicy, ClusterCleanupPolicy, PolicyException)
	// have their execution and admission writes blocked in Kyverno 1.20+. Attempting to rewrite their
	// objects will fail with admission denial. Skip updating them and preserve their existing storedVersions
	// until deleted by cluster operators.
	if deprecations.IsLegacyPolicyKind(crd.Spec.Group, crd.Spec.Names.Kind) {
		fmt.Printf("crd %s is a legacy policy kind whose writes are blocked in Kyverno 1.20+, skipping storage migration\n", crd.Name)
		return nil
	}

	gvr := schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  storedVersion.Name,
		Resource: crd.Spec.Names.Plural,
	}
	resource := dynamicClient.Resource(gvr)
	var client dynamic.ResourceInterface
	if crd.Spec.Scope == v1.NamespaceScoped {
		client = resource.Namespace(metav1.NamespaceAll)
	} else {
		client = resource
	}

	fmt.Println("migrating resources...")
	list, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		// If the resource was removed between CRD Get and resource List, skip cleanly.
		if apierrors.IsNotFound(err) {
			fmt.Printf("resource %s not found on cluster, skipping\n", gvr.Resource)
			return nil
		}
		return err
	}

	for i := range list.Items {
		var itemClient dynamic.ResourceInterface
		if crd.Spec.Scope == v1.NamespaceScoped {
			itemClient = resource.Namespace(list.Items[i].GetNamespace())
		} else {
			itemClient = resource
		}
		_, err := itemClient.Update(ctx, &list.Items[i], metav1.UpdateOptions{})
		if err != nil {
			// If the object was deleted since the list call, it is gone from etcd
			// and does not need storage migration.
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to update %s %s: %w", gvr.Resource, list.Items[i].GetName(), err)
		}
	}

	fmt.Println("patching status...")
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latestCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if (crd.UID != "" && latestCrd.UID != crd.UID) || latestCrd.Generation != crd.Generation {
			return fmt.Errorf("crd %s was modified during migration (UID or generation changed), please rerun migration", crd.GetName())
		}
		var latestStoredVersion *v1.CustomResourceDefinitionVersion
		for i := range latestCrd.Spec.Versions {
			if latestCrd.Spec.Versions[i].Storage {
				latestStoredVersion = &latestCrd.Spec.Versions[i]
			}
		}
		if latestStoredVersion == nil {
			return errors.New("stored version not found")
		}
		if latestStoredVersion.Name != storedVersion.Name {
			return fmt.Errorf("crd %s storage version changed from %s to %s during migration, please rerun migration", crd.GetName(), storedVersion.Name, latestStoredVersion.Name)
		}
		latestCrd.Status.StoredVersions = []string{latestStoredVersion.Name}
		_, err = apiServerClient.ApiextensionsV1().CustomResourceDefinitions().UpdateStatus(ctx, latestCrd, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Printf("crd %s was removed before status update, skipping\n", crd.GetName())
			return nil
		}
		return err
	}
	return nil
}
