package v1

import (
	"testing"

	"gotest.tools/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_MatchResources(t *testing.T) {
	testCases := []struct {
		name       string
		namespaced bool
		subject    MatchResources
		errors     []string
	}{{
		name:       "valid",
		namespaced: true,
		subject: MatchResources{
			Any: ResourceFilters{{
				UserInfo: UserInfo{
					Subjects: []rbacv1.Subject{{
						Kind:      "ServiceAccount",
						Namespace: "ns",
						Name:      "sa-1",
					}},
				},
			}},
		},
	}, {
		name:       "any-all",
		namespaced: true,
		subject: MatchResources{
			Any: ResourceFilters{{
				UserInfo: UserInfo{
					Subjects: []rbacv1.Subject{{
						Kind:      "ServiceAccount",
						Namespace: "ns",
						Name:      "sa-1",
					}},
				},
			}},
			All: ResourceFilters{{
				UserInfo: UserInfo{
					Subjects: []rbacv1.Subject{{
						Kind:      "ServiceAccount",
						Namespace: "ns",
						Name:      "sa-1",
					}},
				},
			}},
		},
		errors: []string{
			`dummy: Invalid value: v1.MatchResources{Any:v1.ResourceFilters{v1.ResourceFilter{UserInfo:v1.UserInfo{Roles:[]string(nil), ClusterRoles:[]string(nil), Subjects:[]v1.Subject{v1.Subject{Kind:"ServiceAccount", APIGroup:"", Name:"sa-1", Namespace:"ns"}}}, ResourceDescription:v1.ResourceDescription{Kinds:[]string(nil), Name:"", Names:[]string(nil), Namespaces:[]string(nil), Annotations:map[string]string(nil), Selector:(*v1.LabelSelector)(nil), NamespaceSelector:(*v1.LabelSelector)(nil), Operations:[]v1.AdmissionOperation(nil)}}}, All:v1.ResourceFilters{v1.ResourceFilter{UserInfo:v1.UserInfo{Roles:[]string(nil), ClusterRoles:[]string(nil), Subjects:[]v1.Subject{v1.Subject{Kind:"ServiceAccount", APIGroup:"", Name:"sa-1", Namespace:"ns"}}}, ResourceDescription:v1.ResourceDescription{Kinds:[]string(nil), Name:"", Names:[]string(nil), Namespaces:[]string(nil), Annotations:map[string]string(nil), Selector:(*v1.LabelSelector)(nil), NamespaceSelector:(*v1.LabelSelector)(nil), Operations:[]v1.AdmissionOperation(nil)}}}, UserInfo:v1.UserInfo{Roles:[]string(nil), ClusterRoles:[]string(nil), Subjects:[]v1.Subject(nil)}, ResourceDescription:v1.ResourceDescription{Kinds:[]string(nil), Name:"", Names:[]string(nil), Namespaces:[]string(nil), Annotations:map[string]string(nil), Selector:(*v1.LabelSelector)(nil), NamespaceSelector:(*v1.LabelSelector)(nil), Operations:[]v1.AdmissionOperation(nil)}}: Can't specify any and all together`,
		},
	}}

	path := field.NewPath("dummy")
	for _, testCase := range testCases {
		errs := testCase.subject.Validate(path, testCase.namespaced, nil)
		assert.Equal(t, len(errs), len(testCase.errors))
		for i, err := range errs {
			assert.Equal(t, err.Error(), testCase.errors[i])
		}
	}
}
