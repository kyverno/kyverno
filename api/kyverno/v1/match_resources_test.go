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
func TestMatchResources_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		mr   MatchResources
		want bool
	}{
		{
			name: "empty match resources",
			mr:   MatchResources{},
			want: true,
		},
		{
			name: "not empty - has any filter",
			mr: MatchResources{
				Any: ResourceFilters{
					{
						UserInfo: UserInfo{
							Roles: []string{"admin"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "not empty - has all filter",
			mr: MatchResources{
				All: ResourceFilters{
					{
						ResourceDescription: ResourceDescription{
							Kinds: []string{"Pod"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "not empty - has resource description",
			mr: MatchResources{
				ResourceDescription: ResourceDescription{
					Kinds: []string{"Deployment"},
				},
			},
			want: false,
		},
		{
			name: "not empty - has user info",
			mr: MatchResources{
				UserInfo: UserInfo{
					Roles: []string{"admin"},
				},
			},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.mr.IsEmpty(); got != tc.want {
				t.Errorf("MatchResources.IsEmpty() = %v, want %v", got, tc.want)
			}
		})
	}
}