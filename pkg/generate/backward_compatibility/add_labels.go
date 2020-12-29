package backwardcompatibility

import (
	"context"
	"fmt"

	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// AddLabels - adds labels to all the existing generate requests
func AddLabels(client *kyvernoclient.Clientset, grInformer kyvernoinformer.GenerateRequestInformer) {
	// Get all the GR's that are existing
	// Extract and Update all of them with the with the labels
	grList, err := grInformer.Lister().List(labels.NewSelector())
	if err != nil {
		// throw some error!
		fmt.Println("error occurred while getting gr list")
		fmt.Println(err)
	}

	for _, gr := range grList {
		gr.SetLabels(map[string]string{
			"generate.kyverno.io/policy-name":        gr.Spec.Policy,
			"generate.kyverno.io/resource-name":      gr.Spec.Resource.Name,
			"generate.kyverno.io/resource-kind":      gr.Spec.Resource.Kind,
			"generate.kyverno.io/resource-namespace": gr.Spec.Resource.Namespace,
		})

		_, err = client.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Update(context.TODO(), gr, metav1.UpdateOptions{})
		if err != nil {
			fmt.Println("error occured while updating gr", gr.Name)
			fmt.Println(err)
		}
	}
}
