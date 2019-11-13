package engine

import (
	"flag"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
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
		info     RequestInfo
		expected bool
	}{
		{
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{},
			},
			info:     RequestInfo{},
			expected: true,
		},
		{
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					Roles: []string{"ns-a:role-a"},
				},
			},
			info: RequestInfo{
				Roles: []string{"ns-a:role-a"},
			},
			expected: true,
		},
		{
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					Roles: []string{"ns-a:role-a"},
				},
			},
			info: RequestInfo{
				Roles: []string{"ns-a:role"},
			},
			expected: false,
		},
		{
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					Subjects: testSubjects(),
				},
			},
			info: RequestInfo{
				AdmissionUserInfo: authenticationv1.UserInfo{
					Username: "serviceaccount:mynamespace:mysa",
				},
			},
			expected: false,
		},
		{
			rule: kyverno.Rule{
				ExcludeResources: kyverno.ExcludeResources{
					Subjects: testSubjects(),
				},
			},
			info: RequestInfo{
				AdmissionUserInfo: authenticationv1.UserInfo{
					UID: "1",
				},
			},
			expected: true,
		},
		{
			rule: kyverno.Rule{
				ExcludeResources: kyverno.ExcludeResources{
					Subjects: testSubjects(),
				},
			},
			info: RequestInfo{
				AdmissionUserInfo: authenticationv1.UserInfo{
					Username: "kubernetes-admin",
					Groups:   []string{"system:masters", "system:authenticated"},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		assert.Assert(t, test.expected == matchAdmissionInfo(test.rule, test.info))
	}
}

func Test_validateMatch(t *testing.T) {
	requestInfo := []struct {
		info     RequestInfo
		expected bool
	}{
		{
			info: RequestInfo{
				Roles: []string{},
			},
			expected: false,
		},
		{
			info: RequestInfo{
				Roles: []string{"ns-b:role-b"},
			},
			expected: true,
		},
		{
			info: RequestInfo{
				Roles: []string{"ns:role"},
			},
			expected: false,
		},
	}

	matchRoles := kyverno.MatchResources{
		Roles: []string{"ns-a:role-a", "ns-b:role-b"},
	}

	for _, info := range requestInfo {
		assert.Assert(t, info.expected == validateMatch(matchRoles, info.info))
	}

	requestInfo = []struct {
		info     RequestInfo
		expected bool
	}{
		{
			info: RequestInfo{
				ClusterRoles: []string{},
			},
			expected: false,
		},
		{
			info: RequestInfo{
				ClusterRoles: []string{"role-b"},
			},
			expected: false,
		},
		{
			info: RequestInfo{
				ClusterRoles: []string{"clusterrole-b"},
			},
			expected: true,
		},
		{
			info: RequestInfo{
				ClusterRoles: []string{"clusterrole-a", "clusterrole-b"},
			},
			expected: true,
		},
		{
			info: RequestInfo{
				ClusterRoles: []string{"fake-a", "fake-b"},
			},
			expected: false,
		},
	}

	matchClusterRoles := kyverno.MatchResources{
		ClusterRoles: []string{"clusterrole-a", "clusterrole-b"},
	}

	for _, info := range requestInfo {
		assert.Assert(t, info.expected == validateMatch(matchClusterRoles, info.info))
	}
}

func Test_validateExclude(t *testing.T) {
	requestInfo := []struct {
		info     RequestInfo
		expected bool
	}{
		{
			info: RequestInfo{
				Roles: []string{},
			},
			expected: true,
		},
		{
			info: RequestInfo{
				Roles: []string{"ns-b:role-b"},
			},
			expected: false,
		},
		{
			info: RequestInfo{
				Roles: []string{"ns:role"},
			},
			expected: true,
		},
	}

	excludeRoles := kyverno.ExcludeResources{
		Roles: []string{"ns-a:role-a", "ns-b:role-b"},
	}

	for _, info := range requestInfo {
		assert.Assert(t, info.expected == validateExclude(excludeRoles, info.info))
	}

	requestInfo = []struct {
		info     RequestInfo
		expected bool
	}{
		{
			info: RequestInfo{
				ClusterRoles: []string{},
			},
			expected: true,
		},
		{
			info: RequestInfo{
				ClusterRoles: []string{"role-b"},
			},
			expected: true,
		},
		{
			info: RequestInfo{
				ClusterRoles: []string{"clusterrole-b"},
			},
			expected: false,
		},
		{
			info: RequestInfo{
				ClusterRoles: []string{"fake-a", "fake-b"},
			},
			expected: true,
		},
	}

	excludeClusterRoles := kyverno.ExcludeResources{
		ClusterRoles: []string{"clusterrole-a", "clusterrole-b"},
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
