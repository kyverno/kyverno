package generate

import (
	"fmt"
	"testing"
	"time"

	commonE2E "github.com/kyverno/kyverno/test/e2e/common"

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
			expectResourcesNotExist(e2eClient, test.ExpectedResources...)

			// create source resources
			createResources(t, e2eClient, test.SourceResources...)

			// create policy
			policy := createResource(t, e2eClient, test.ClusterPolicy)
			Expect(commonE2E.PolicyCreated(policy.GetName())).To(Succeed())

			// create trigger
			createResource(t, e2eClient, test.TriggerResource)

			time.Sleep(time.Second * 5)

			for _, step := range test.Steps {
				Expect(step(e2eClient)).To(Succeed())
			}

			// verify expected resources
			expectResources(e2eClient, test.ExpectedResources...)
		})
	}
}

func Test_ClusterRole_ClusterRoleBinding_Sets(t *testing.T) {
	runTestCases(t, ClusterRoleTests...)
}

func Test_Role_RoleBinding_Sets(t *testing.T) {
	runTestCases(t, RoleTests...)
}

func Test_Generate_NetworkPolicy(t *testing.T) {
	runTestCases(t, NetworkPolicyGenerateTests...)
}

func Test_Generate_Namespace_Label_Actions(t *testing.T) {
	runTestCases(t, GenerateNetworkPolicyOnNamespaceWithoutLabelTests...)
}

func loopElement(found bool, elementObj interface{}) bool {
	if found == true {
		return found
	}
	switch typedelementObj := elementObj.(type) {
	case map[string]interface{}:
		for k, v := range typedelementObj {
			if k == "protocol" {
				if v == "TCP" {
					found = true
					return found
				}
			} else {
				found = loopElement(found, v)
			}
		}
	case []interface{}:
		found = loopElement(found, typedelementObj[0])
	case string:
		return found
	case int64:
		return found
	default:
		fmt.Println("unexpected type :", fmt.Sprintf("%T", elementObj))
		return found
	}
	return found
}

func Test_Generate_Synchronize_Flag(t *testing.T) {
	runTestCases(t, GenerateSynchronizeFlagTests...)
}

func Test_Source_Resource_Update_Replication(t *testing.T) {
	runTestCases(t, SourceResourceUpdateReplicationTests...)
}

func Test_Generate_Policy_Deletion_for_Clone(t *testing.T) {
	runTestCases(t, GeneratePolicyDeletionforCloneTests...)
}
