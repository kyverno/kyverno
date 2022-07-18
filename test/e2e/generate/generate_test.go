package generate

import (
	"testing"
	"time"

	commonE2E "github.com/kyverno/kyverno/test/e2e/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func runTestCases(t *testing.T, testCases ...testCase) {
	setup(t)

	for _, test := range testCases {
		t.Run(test.TestName, func(t *testing.T) {
			e2eClient := createClient()

			t.Cleanup(func() {
				deleteResources(e2eClient, test.ExpectedResources...)
			})

			// sanity check
			By("Verifying expected resources do not exist yet in the cluster ...")
			expectResourcesNotFound(e2eClient, test.ExpectedResources...)

			// create source resources
			if len(test.SourceResources) > 0 {
				By("Creating source resources ...")
				createResources(t, e2eClient, test.SourceResources...)
			}

			// create policy
			By("Creating cluster policy ...")
			policy := createResource(t, e2eClient, test.ClusterPolicy)
			Expect(commonE2E.PolicyCreated(policy.GetName())).To(Succeed())

			// create trigger
			By("Creating trigger resource ...")
			createResource(t, e2eClient, test.TriggerResource)

			time.Sleep(time.Second * 5)

			for _, step := range test.Steps {
				Expect(step(e2eClient)).To(Succeed())
			}

			// verify expected resources
			By("Verifying resource expectations ...")
			expectResources(e2eClient, test.ExpectedResources...)
		})
	}
}

func Test_ClusterRole_ClusterRoleBinding_Sets(t *testing.T) {
	runTestCases(t, clusterRoleTests...)
}

func Test_Role_RoleBinding_Sets(t *testing.T) {
	runTestCases(t, roleTests...)
}

func Test_Generate_NetworkPolicy(t *testing.T) {
	runTestCases(t, networkPolicyGenerateTests...)
}

func Test_Generate_Namespace_Label_Actions(t *testing.T) {
	runTestCases(t, generateNetworkPolicyOnNamespaceWithoutLabelTests...)
}

func Test_Generate_Synchronize_Flag(t *testing.T) {
	runTestCases(t, generateSynchronizeFlagTests...)
}

func Test_Source_Resource_Update_Replication(t *testing.T) {
	runTestCases(t, sourceResourceUpdateReplicationTests...)
}

func Test_Generate_Policy_Deletion_for_Clone(t *testing.T) {
	runTestCases(t, generatePolicyDeletionforCloneTests...)
}
