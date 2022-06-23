package step

import (
	"fmt"

	"github.com/kyverno/kyverno/test/e2e/framework/client"
	"github.com/kyverno/kyverno/test/e2e/framework/id"
	"github.com/kyverno/kyverno/test/e2e/framework/utils"
	"github.com/onsi/ginkgo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ResourceExpectation func(*unstructured.Unstructured)

func ExpectResource(id id.Id, expectations ...ResourceExpectation) Step {
	return func(client client.Client) {
		ginkgo.By(fmt.Sprintf("Checking resource expectations (%s : %s) ...", id.GetGvr(), utils.Key(id)))
		resource := client.GetResource(id)
		for _, expectation := range expectations {
			expectation(resource)
		}
	}
}
