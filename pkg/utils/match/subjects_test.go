package match

import (
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func TestCheckSubjects(t *testing.T) {
	tests := []struct {
		name         string
		ruleSubjects []rbacv1.Subject
		userInfo     authenticationv1.UserInfo
		want         bool
	}{
		//For null cases
		{
			name:         "empty subjects returns false",
			ruleSubjects: []rbacv1.Subject{},
			userInfo: authenticationv1.UserInfo{
				Username: "admin",
			},
			want: false,
		},
		{
			name:         "nil subjects returns false",
			ruleSubjects: nil,
			userInfo: authenticationv1.UserInfo{
				Username: "admin",
			},
			want: false,
		},

		// For service account matching
		{
			name: "ServiceAccount exact match",
			ruleSubjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "default",
				Namespace: "kube-system",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:default",
			},
			want: true,
		},
		{
			name: "ServiceAccount no match - different namespace",
			ruleSubjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "default",
				Namespace: "kube-system",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:default:default",
			},
			want: false,
		},
		{
			name: "ServiceAccount no match - different name",
			ruleSubjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "admin",
				Namespace: "kube-system",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:default",
			},
			want: false,
		},
		{
			name: "ServiceAccount wildcard name match",
			ruleSubjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "*",
				Namespace: "kube-system",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:any-sa",
			},
			want: true,
		},
		{
			name: "ServiceAccount wildcard namespace via pattern",
			ruleSubjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "kyverno",
				Namespace: "kyverno*",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kyverno-system:kyverno",
			},
			want: true,
		},

		// User matching
		{
			name: "User exact match",
			ruleSubjects: []rbacv1.Subject{{
				Kind: rbacv1.UserKind,
				Name: "alice",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "alice",
			},
			want: true,
		},
		{
			name: "User no match",
			ruleSubjects: []rbacv1.Subject{{
				Kind: rbacv1.UserKind,
				Name: "alice",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "bob",
			},
			want: false,
		},
		{
			name: "User wildcard match",
			ruleSubjects: []rbacv1.Subject{{
				Kind: rbacv1.UserKind,
				Name: "admin-*",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "admin-alice",
			},
			want: true,
		},
		{
			name: "User wildcard no match",
			ruleSubjects: []rbacv1.Subject{{
				Kind: rbacv1.UserKind,
				Name: "admin-*",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "user-alice",
			},
			want: false,
		},

		// To match grps
		{
			name: "Group exact match",
			ruleSubjects: []rbacv1.Subject{{
				Kind: rbacv1.GroupKind,
				Name: "system:masters",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "admin",
				Groups:   []string{"system:authenticated", "system:masters"},
			},
			want: true,
		},
		{
			name: "Group no match",
			ruleSubjects: []rbacv1.Subject{{
				Kind: rbacv1.GroupKind,
				Name: "system:masters",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "user",
				Groups:   []string{"system:authenticated"},
			},
			want: false,
		},
		{
			name: "Group wildcard match",
			ruleSubjects: []rbacv1.Subject{{
				Kind: rbacv1.GroupKind,
				Name: "system:*",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "user",
				Groups:   []string{"developers", "system:authenticated"},
			},
			want: true,
		},
		{
			name: "Group empty groups in userInfo",
			ruleSubjects: []rbacv1.Subject{{
				Kind: rbacv1.GroupKind,
				Name: "admin",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "user",
				Groups:   []string{},
			},
			want: false,
		},
		{
			name: "Group nil groups in userInfo",
			ruleSubjects: []rbacv1.Subject{{
				Kind: rbacv1.GroupKind,
				Name: "admin",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "user",
				Groups:   nil,
			},
			want: false,
		},

		// For multiple subjects
		{
			name: "Multiple subjects - first matches",
			ruleSubjects: []rbacv1.Subject{
				{
					Kind: rbacv1.UserKind,
					Name: "alice",
				},
				{
					Kind: rbacv1.UserKind,
					Name: "bob",
				},
			},
			userInfo: authenticationv1.UserInfo{
				Username: "alice",
			},
			want: true,
		},
		{
			name: "Multiple subjects - second matches",
			ruleSubjects: []rbacv1.Subject{
				{
					Kind: rbacv1.UserKind,
					Name: "alice",
				},
				{
					Kind: rbacv1.UserKind,
					Name: "bob",
				},
			},
			userInfo: authenticationv1.UserInfo{
				Username: "bob",
			},
			want: true,
		},
		{
			name: "Multiple subjects - none match",
			ruleSubjects: []rbacv1.Subject{
				{
					Kind: rbacv1.UserKind,
					Name: "alice",
				},
				{
					Kind: rbacv1.UserKind,
					Name: "bob",
				},
			},
			userInfo: authenticationv1.UserInfo{
				Username: "charlie",
			},
			want: false,
		},

		// Mixed subject types
		{
			name: "Mixed types - User matches",
			ruleSubjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "default",
					Namespace: "kube-system",
				},
				{
					Kind: rbacv1.UserKind,
					Name: "alice",
				},
				{
					Kind: rbacv1.GroupKind,
					Name: "admins",
				},
			},
			userInfo: authenticationv1.UserInfo{
				Username: "alice",
				Groups:   []string{"developers"},
			},
			want: true,
		},
		{
			name: "Mixed types - Group matches",
			ruleSubjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "default",
					Namespace: "kube-system",
				},
				{
					Kind: rbacv1.UserKind,
					Name: "alice",
				},
				{
					Kind: rbacv1.GroupKind,
					Name: "admins",
				},
			},
			userInfo: authenticationv1.UserInfo{
				Username: "bob",
				Groups:   []string{"admins", "developers"},
			},
			want: true,
		},
		{
			name: "Mixed types - ServiceAccount matches",
			ruleSubjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "kyverno",
					Namespace: "kyverno",
				},
				{
					Kind: rbacv1.UserKind,
					Name: "alice",
				},
			},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kyverno:kyverno",
				Groups:   []string{"system:serviceaccounts"},
			},
			want: true,
		},

		// Edge cases
		{
			name: "Unknown subject kind is ignored",
			ruleSubjects: []rbacv1.Subject{{
				Kind: "UnknownKind",
				Name: "test",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "test",
			},
			want: false,
		},
		{
			name: "ServiceAccount with empty namespace",
			ruleSubjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "default",
				Namespace: "",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount::default",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckSubjects(tt.ruleSubjects, tt.userInfo)
			if got != tt.want {
				t.Errorf("CheckSubjects() = %v, want %v", got, tt.want)
			}
		})
	}
}
