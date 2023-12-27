package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/spf13/cobra"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type options struct {
	KubeConfig string
	Context    string
}

func Command() *cobra.Command {
	var options options
	cmd := &cobra.Command{
		Use: "migrate",
		// Short:        command.FormatDescription(true, websiteUrl, false, description...),
		// Long:         command.FormatDescription(false, websiteUrl, false, description...),
		// Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientConfig, err := config.CreateClientConfigWithContext(options.KubeConfig, options.Context)
			if err != nil {
				return err
			}
			apiServerClient, err := clientset.NewForConfig(clientConfig)
			if err != nil {
				return err
			}
			ctx := context.Background()
			crd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, "policyexceptions.kyverno.io", metav1.GetOptions{})
			if err != nil {
				return err
			}
			dynamicClient, err := dynamic.NewForConfig(clientConfig)
			if err != nil {
				return err
			}
			return migrate(ctx, crd, dynamicClient)
		},
	}
	cmd.Flags().StringVar(&options.KubeConfig, "kubeconfig", "", "path to kubeconfig file with authorization and master location information")
	cmd.Flags().StringVar(&options.Context, "context", "", "The name of the kubeconfig context to use")
	return cmd
}

func migrate(ctx context.Context, crd *v1.CustomResourceDefinition, dynamicClient *dynamic.DynamicClient) error {
	var storedVersion *v1.CustomResourceDefinitionVersion
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Storage {
			storedVersion = &crd.Spec.Versions[i]
		}
	}
	if storedVersion == nil {
		return errors.New("stored version not found")
	} else {
		fmt.Println("Stored version", storedVersion.Name)
		gvr := schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  storedVersion.Name,
			Resource: crd.Spec.Names.Plural,
		}
		fmt.Println("GVR", gvr)
		resource := dynamicClient.Resource(gvr)
		var client dynamic.ResourceInterface
		if crd.Spec.Scope == v1.NamespaceScoped {
			client = resource.Namespace(metav1.NamespaceAll)
		} else {
			client = resource
		}
		list, err := client.List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		fmt.Println("List", list.Items)
		for i := range list.Items {
			var client dynamic.ResourceInterface
			if crd.Spec.Scope == v1.NamespaceScoped {
				client = resource.Namespace(list.Items[i].GetNamespace())
			} else {
				client = resource
			}
			_, err := client.Update(ctx, &list.Items[i], metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
