package rbac

import (
	"flag"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"gotest.tools/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func Test_matchAdmissionInfo(t *testing.T) {
	flag.Parse()
	flag.Set("logtostderr", "true")
	flag.Set("v", "3")
	tests := []struct {
		rule     kyverno.Rule
		info     kyverno.RequestInfo
		expected bool
	}{
		{
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{},
			},
			info:     kyverno.RequestInfo{},
			expected: true,
		},
		{
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					UserInfo: kyverno.UserInfo{
						Roles: []string{"ns-a:role-a"},
					},
				},
			},
			info: kyverno.RequestInfo{
				Roles: []string{"ns-a:role-a"},
			},
			expected: true,
		},
		{
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					UserInfo: kyverno.UserInfo{
						Roles: []string{"ns-a:role-a"},
					},
				},
			},
			info: kyverno.RequestInfo{
				Roles: []string{"ns-a:role"},
			},
			expected: false,
		},
		{
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					UserInfo: kyverno.UserInfo{
						Subjects: testSubjects(),
					},
				},
			},
			info: kyverno.RequestInfo{
				AdmissionUserInfo: authenticationv1.UserInfo{
					Username: "serviceaccount:mynamespace:mysa",
				},
			},
			expected: false,
		},
		{
			rule: kyverno.Rule{
				ExcludeResources: kyverno.ExcludeResources{
					UserInfo: kyverno.UserInfo{
						Subjects: testSubjects(),
					},
				},
			},
			info: kyverno.RequestInfo{
				AdmissionUserInfo: authenticationv1.UserInfo{
					UID: "1",
				},
			},
			expected: true,
		},
		{
			rule: kyverno.Rule{
				ExcludeResources: kyverno.ExcludeResources{
					UserInfo: kyverno.UserInfo{
						Subjects: testSubjects(),
					},
				},
			},
			info: kyverno.RequestInfo{
				AdmissionUserInfo: authenticationv1.UserInfo{
					Username: "kubernetes-admin",
					Groups:   []string{"system:masters", "system:authenticated"},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		assert.Assert(t, test.expected == MatchAdmissionInfo(test.rule, test.info))
	}
}

func Test_validateMatch(t *testing.T) {
	requestInfo := []struct {
		info     kyverno.RequestInfo
		expected bool
	}{
		{
			info: kyverno.RequestInfo{
				Roles: []string{},
			},
			expected: false,
		},
		{
			info: kyverno.RequestInfo{
				Roles: []string{"ns-b:role-b"},
			},
			expected: true,
		},
		{
			info: kyverno.RequestInfo{
				Roles: []string{"ns:role"},
			},
			expected: false,
		},
	}

	matchRoles := kyverno.MatchResources{
		UserInfo: kyverno.UserInfo{
			Roles: []string{"ns-a:role-a", "ns-b:role-b"},
		},
	}

	for _, info := range requestInfo {
		assert.Assert(t, info.expected == validateMatch(matchRoles, info.info))
	}

	requestInfo = []struct {
		info     kyverno.RequestInfo
		expected bool
	}{
		{
			info: kyverno.RequestInfo{
				ClusterRoles: []string{},
			},
			expected: false,
		},
		{
			info: kyverno.RequestInfo{
				ClusterRoles: []string{"role-b"},
			},
			expected: false,
		},
		{
			info: kyverno.RequestInfo{
				ClusterRoles: []string{"clusterrole-b"},
			},
			expected: true,
		},
		{
			info: kyverno.RequestInfo{
				ClusterRoles: []string{"clusterrole-a", "clusterrole-b"},
			},
			expected: true,
		},
		{
			info: kyverno.RequestInfo{
				ClusterRoles: []string{"fake-a", "fake-b"},
			},
			expected: false,
		},
	}

	matchClusterRoles := kyverno.MatchResources{
		UserInfo: kyverno.UserInfo{
			ClusterRoles: []string{"clusterrole-a", "clusterrole-b"},
		},
	}

	for _, info := range requestInfo {
		assert.Assert(t, info.expected == validateMatch(matchClusterRoles, info.info))
	}
}

func Test_validateExclude(t *testing.T) {
	requestInfo := []struct {
		info     kyverno.RequestInfo
		expected bool
	}{
		{
			info: kyverno.RequestInfo{
				Roles: []string{},
			},
			expected: true,
		},
		{
			info: kyverno.RequestInfo{
				Roles: []string{"ns-b:role-b"},
			},
			expected: false,
		},
		{
			info: kyverno.RequestInfo{
				Roles: []string{"ns:role"},
			},
			expected: true,
		},
	}

	excludeRoles := kyverno.ExcludeResources{
		UserInfo: kyverno.UserInfo{
			Roles: []string{"ns-a:role-a", "ns-b:role-b"},
		},
	}

	for _, info := range requestInfo {
		assert.Assert(t, info.expected == validateExclude(excludeRoles, info.info))
	}

	requestInfo = []struct {
		info     kyverno.RequestInfo
		expected bool
	}{
		{
			info: kyverno.RequestInfo{
				ClusterRoles: []string{},
			},
			expected: true,
		},
		{
			info: kyverno.RequestInfo{
				ClusterRoles: []string{"role-b"},
			},
			expected: true,
		},
		{
			info: kyverno.RequestInfo{
				ClusterRoles: []string{"clusterrole-b"},
			},
			expected: false,
		},
		{
			info: kyverno.RequestInfo{
				ClusterRoles: []string{"fake-a", "fake-b"},
			},
			expected: true,
		},
	}

	excludeClusterRoles := kyverno.ExcludeResources{
		UserInfo: kyverno.UserInfo{
			ClusterRoles: []string{"clusterrole-a", "clusterrole-b"},
		},
	}

	for _, info := range requestInfo {
		assert.Assert(t, info.expected == validateExclude(excludeClusterRoles, info.info))
	}
}

func Test_matchSubjects(t *testing.T) {
	group := authenticationv1.UserInfo{
		Username: "kubernetes-admin",
		Groups:   []string{"system:masters", "system:authenticated"},
	}

	sa := authenticationv1.UserInfo{
		Username: "system:serviceaccount:mynamespace:mysa",
		Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:mynamespace", "system:authenticated"},
	}

	user := authenticationv1.UserInfo{
		Username: "system:kube-scheduler",
		Groups:   []string{"system:authenticated"},
	}

	subjects := testSubjects()

	assert.Assert(t, matchSubjects(subjects, sa))
	assert.Assert(t, !matchSubjects(subjects, user))
	assert.Assert(t, matchSubjects(subjects, group))
}

func testSubjects() []rbacv1.Subject {
	return []rbacv1.Subject{
		{
			Kind: "User",
			Name: "kube-scheduler",
		},
		{
			Kind: "Group",
			Name: "system:masters",
		},
		{
			Kind:      "ServiceAccount",
			Name:      "mysa",
			Namespace: "mynamespace",
		},
	}
}
