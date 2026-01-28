package v1

import (
	"testing"
)

func TestResourceFilter_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		rf   ResourceFilter
		want bool
	}{
		{
			name: "empty filter",
			rf:   ResourceFilter{},
			want: true,
		},
		{
			name: "not empty - has user info",
			rf: ResourceFilter{
				UserInfo: UserInfo{
					Roles: []string{"admin"},
				},
			},
			want: false,
		},
		{
			name: "not empty - has resource description",
			rf: ResourceFilter{
				ResourceDescription: ResourceDescription{
					Kinds: []string{"Pod"},
				},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rf.IsEmpty(); got != tc.want {
				t.Errorf("ResourceFilter.IsEmpty() = %v, want %v", got, tc.want)
			}
		})
	}
}
