package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/spf13/cobra"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)

type options struct {
	KubeConfig string
	Context    string
	Resources  []string
}

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

func migrate(ctx context.Context, crd *v1.CustomResourceDefinition, dynamicClient dynamic.Interface, apiServerClient clientset.Interface) error {
	var storedVersion *v1.CustomResourceDefinitionVersion
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Storage {
			storedVersion = &crd.Spec.Versions[i]
		}
	}
	if storedVersion == nil {
		return errors.New("stored version not found")
	} else {
		fmt.Println("stored version:", storedVersion.Name)
		if len(crd.Status.StoredVersions) == 1 {
			if crd.Status.StoredVersions[0] == storedVersion.Name {
				fmt.Println("stored version is already up to date, nothing to do")
				return nil
			}
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
			return err
		}
		for i := range list.Items {
			var client dynamic.ResourceInterface
			if crd.Spec.Scope == v1.NamespaceScoped {
				client = resource.Namespace(list.Items[i].GetNamespace())
			} else {
				client = resource
			}
			_, err := client.Update(ctx, &list.Items[i], metav1.UpdateOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				if apierrors.IsForbidden(err) || apierrors.IsInvalid(err) {
					fmt.Printf("warning: unable to update %s %s: %v (skipping)\n", gvr.Resource, list.Items[i].GetName(), err)
					continue
				}
				return err
			}
		}
		fmt.Println("patching status...")
		return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			latestCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			latestCrd.Status.StoredVersions = []string{storedVersion.Name}
			_, err = apiServerClient.ApiextensionsV1().CustomResourceDefinitions().UpdateStatus(ctx, latestCrd, metav1.UpdateOptions{})
			return err
		})
	}
}
