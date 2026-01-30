package v1

import (
	"strings"
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
	}{
		{
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
		},
		{
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
				"Can't specify any and all together",
			},
		}}

	path := field.NewPath("dummy")
	for _, testCase := range testCases {
		errs := testCase.subject.Validate(path, testCase.namespaced, nil)
		assert.Equal(t, len(errs), len(testCase.errors))
		for i, err := range errs {
			assert.Assert(t, strings.Contains(err.Error(), testCase.errors[i]))
		}
	}
}

func TestMatchResources_Validate_Empty_NoPanic(t *testing.T) {
	mr := MatchResources{}

	path := field.NewPath("dummy")
	errs := mr.Validate(path, false, nil)

	if len(errs) > 0 {
		t.Logf("Empty validation returned errors (expected behavior): %v", errs)
	}
}
