package v1alpha1

import (
	"encoding/json"
	"fmt"
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_CleanupPolicy_Name(t *testing.T) {
	subject := CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "this-is-a-way-too-long-policy-name-that-should-trigger-an-error-when-calling-the-policy-validation-method",
		},
		Spec: CleanupPolicySpec{
			Schedule: "* * * * *",
		},
	}
	errs := subject.Validate(nil)
	assert.Assert(t, len(errs) == 1)
	assert.Equal(t, errs[0].Field, "metadata.name")
	assert.Equal(t, errs[0].Type, field.ErrorTypeTooLong)
	assert.Equal(t, errs[0].Detail, "must have at most 63 bytes")
	assert.Equal(t, errs[0].Error(), "metadata.name: Too long: must have at most 63 bytes")
}

func Test_CleanupPolicy_Schedule(t *testing.T) {
	subject := CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: CleanupPolicySpec{
			Schedule: "schedule-not-in-proper-cron-format",
		},
	}
	errs := subject.Validate(nil)
	assert.Assert(t, len(errs) == 1)
	assert.Equal(t, errs[0].Field, "spec.schedule")
	assert.Equal(t, errs[0].Type, field.ErrorTypeInvalid)
	assert.Equal(t, errs[0].Detail, "schedule spec in the cleanupPolicy is not in proper cron format")
	assert.Equal(t, errs[0].Error(), fmt.Sprintf(`spec.schedule: Invalid value: "%s": schedule spec in the cleanupPolicy is not in proper cron format`, subject.Spec.Schedule))
}

func Test_ClusterCleanupPolicy_Name(t *testing.T) {
	subject := ClusterCleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "this-is-a-way-too-long-policy-name-that-should-trigger-an-error-when-calling-the-policy-validation-method",
		},
		Spec: CleanupPolicySpec{
			Schedule: "* * * * *",
		},
	}
	errs := subject.Validate(nil)
	assert.Assert(t, len(errs) == 1)
	assert.Equal(t, errs[0].Field, "metadata.name")
	assert.Equal(t, errs[0].Type, field.ErrorTypeTooLong)
	assert.Equal(t, errs[0].Detail, "must have at most 63 bytes")
	assert.Equal(t, errs[0].Error(), "metadata.name: Too long: must have at most 63 bytes")
}

func Test_ClusterCleanupPolicy_Schedule(t *testing.T) {
	subject := ClusterCleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: CleanupPolicySpec{
			Schedule: "schedule-not-in-proper-cron-format",
		},
	}
	errs := subject.Validate(nil)
	assert.Assert(t, len(errs) == 1)
	assert.Equal(t, errs[0].Field, "spec.schedule")
	assert.Equal(t, errs[0].Type, field.ErrorTypeInvalid)
	assert.Equal(t, errs[0].Detail, "schedule spec in the cleanupPolicy is not in proper cron format")
	assert.Equal(t, errs[0].Error(), fmt.Sprintf(`spec.schedule: Invalid value: "%s": schedule spec in the cleanupPolicy is not in proper cron format`, subject.Spec.Schedule))
}

func Test_doesMatchExcludeConflict(t *testing.T) {
	path := field.NewPath("dummy")
	testcases := []struct {
		description string
		policySpec  []byte
		errors      func(r *CleanupPolicySpec) field.ErrorList
	}{
		{
			description: "Same match and exclude",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}, "schedule": "* * * * *"}`),
			errors: func(r *CleanupPolicySpec) (errs field.ErrorList) {
				return append(errs, field.Invalid(path, r, "CleanupPolicy is matching an empty set"))
			},
		},
		{
			description: "Failed to exclude kind",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude":{"resources":{"kinds":["Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}, "schedule": "* * * * *"}`),
		},
		{
			description: "Failed to exclude name",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude":{"resources":{"kinds":["Pod","Namespace"],"name":"something-*","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}, "schedule": "* * * * *"}`),
		},
		{
			description: "Failed to exclude namespace",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something3","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}, "schedule": "* * * * *"}`),
		},
		{
			description: "Failed to exclude labels",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"higha"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}, "schedule": "* * * * *"}`),
		},
		{
			description: "Failed to exclude expression",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["databases"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}, "schedule": "* * * * *"}`),
		},
		{
			description: "Failed to exclude subjects",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something2","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}, "schedule": "* * * * *"}`),
		},
		{
			description: "Failed to exclude clusterroles",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something3","something1"],"roles":["something","something1"]}, "schedule": "* * * * *"}`),
		},
		{
			description: "Failed to exclude roles",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"]}]}},"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something3","something1"]}, "schedule": "* * * * *"}`),
		},
		{
			description: "simple",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"]}},"exclude":{"resources":{"kinds":["Pod","Namespace","Job"],"name":"some*","namespaces":["something","something1","something2"]}}, "schedule": "* * * * *"}`),
			errors: func(r *CleanupPolicySpec) (errs field.ErrorList) {
				return append(errs, field.Invalid(path, r, "CleanupPolicy is matching an empty set"))
			},
		},
		{
			description: "simple - fail",
			policySpec:  []byte(`{"match":{"resources":{"kinds":["Pod","Namespace"],"name":"somxething","namespaces":["something","something1"]}},"exclude":{"resources":{"kinds":["Pod","Namespace","Job"],"name":"some*","namespaces":["something","something1","something2"]}}, "schedule": "* * * * *"}`),
		},
		{
			description: "empty case",
			policySpec:  []byte(`{"match":{"resources":{"selector":{"matchLabels":{"allow-deletes":"false"}}}},"exclude":{"clusterRoles":["random"]},"validate":{"message":"Deleting {{request.object.kind}}/{{request.object.metadata.name}} is not allowed","deny":{"conditions":{"all":[{"key":"{{request.operation}}","operator":"Equal","value":"DELETE"}]}}}, "schedule": "* * * * *"}`),
		},
	}
	for _, testcase := range testcases {
		var policySpec CleanupPolicySpec
		err := json.Unmarshal(testcase.policySpec, &policySpec)
		assert.NilError(t, err)
		errs := policySpec.ValidateMatchExcludeConflict(path)
		var expectedErrs field.ErrorList
		if testcase.errors != nil {
			expectedErrs = testcase.errors(&policySpec)
		}
		assert.Equal(t, len(errs), len(expectedErrs))
		for i := range errs {
			fmt.Println(i)
			assert.Equal(t, errs[i].Error(), expectedErrs[i].Error())
		}
	}
}
