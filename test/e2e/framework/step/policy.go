package step

import (
	"github.com/kyverno/kyverno/test/e2e/common"
	"github.com/kyverno/kyverno/test/e2e/framework/client"
	"github.com/kyverno/kyverno/test/e2e/framework/resource"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

func CreateClusterPolicy(data []byte) Step {
	return func(client client.Client) {
		ginkgo.By("Creating cluster policy ...")
		policy := client.CreateResource(resource.ClusterPolicy(data))
		gomega.Expect(common.PolicyCreated(policy.GetName())).To(gomega.Succeed())
	}
}
