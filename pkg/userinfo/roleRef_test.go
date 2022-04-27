package userinfo

import (
	"testing"

	"gotest.tools/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_matchServiceAccount_subject_variants(t *testing.T) {
	userInfo := authenticationv1.UserInfo{
		Username: "system:serviceaccount:default:saconfig",
	}

	tests := []struct {
		subject  rbacv1.Subject
		expected bool
	}{
		{
			subject:  rbacv1.Subject{},
			expected: false,
		},
		{
			subject: rbacv1.Subject{
				Kind: "serviceaccount",
			},
			expected: false,
		},
		{
			subject: rbacv1.Subject{
				Kind:      "ServiceAccount",
				Namespace: "testnamespace",
			},
			expected: false,
		},
		{
			subject: rbacv1.Subject{
				Kind:      "ServiceAccount",
				Namespace: "1",
			},
			expected: false,
		},
		{
			subject: rbacv1.Subject{
				Kind:      "ServiceAccount",
				Namespace: "testnamespace",
				Name:      "",
			},
			expected: false,
		},
		{
			subject: rbacv1.Subject{
				Kind:      "ServiceAccount",
				Namespace: "testnamespace",
				Name:      "testname",
			},
			expected: false,
		},
	}

	for _, test := range tests {
		res := matchServiceAccount(test.subject, userInfo, test.subject.Namespace)
		assert.Equal(t, test.expected, res)
	}
}

func Test_matchUserOrGroup(t *testing.T) {
	group := authenticationv1.UserInfo{
		Username: "kubernetes-admin",
		Groups:   []string{"system:masters", "system:authenticated"},
	}

	sa := authenticationv1.UserInfo{
		Username: "system:serviceaccount:kube-system:deployment-controller",
		Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:kube-system", "system:authenticated"},
	}

	user := authenticationv1.UserInfo{
		Username: "system:kube-scheduler",
		Groups:   []string{"system:authenticated"},
	}

	userContext := rbacv1.Subject{
		Kind: "User",
		Name: "system:kube-scheduler",
	}

	groupContext := rbacv1.Subject{
		Kind: "Group",
		Name: "system:masters",
	}

	fakegroupContext := rbacv1.Subject{
		Kind: "Group",
		Name: "fakeGroup",
	}

	res := matchUserOrGroup(userContext, user)
	assert.Assert(t, res)

	res = matchUserOrGroup(groupContext, group)
	assert.Assert(t, res)

	res = matchUserOrGroup(groupContext, sa)
	assert.Assert(t, !res)

	res = matchUserOrGroup(fakegroupContext, group)
	assert.Assert(t, !res)
}

func Test_matchSubjectsMap(t *testing.T) {
	sa := authenticationv1.UserInfo{
		Username: "system:serviceaccount:default:saconfig",
	}

	group := authenticationv1.UserInfo{
		Username: "kubernetes-admin",
		Groups:   []string{"system:masters", "system:authenticated"},
	}

	sasubject := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Namespace: "default",
		Name:      "saconfig",
	}

	groupsubject := rbacv1.Subject{
		Kind: "Group",
		Name: "fakeGroup",
	}

	res := matchSubjectsMap(sasubject, sa, sasubject.Namespace)
	assert.Assert(t, res)

	res = matchSubjectsMap(groupsubject, group, "")
	assert.Assert(t, !res)
}

func newRoleBinding(name, ns string, subjects []rbacv1.Subject, roles rbacv1.RoleRef) *rbacv1.RoleBinding {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Subjects:   subjects,
		RoleRef:    roles,
	}

	rb.Kind = "RoleBinding"
	rb.APIVersion = "rbac.authorization.k8s.io/v1"
	return rb
}

func Test_getRoleRefByRoleBindings(t *testing.T) {

	// flag.Parse()
	// flag.Set("logtostderr", "true")
	// flag.Set("v", "3")

	list := make([]*rbacv1.RoleBinding, 2)

	list[0] = newRoleBinding("test1", "default",
		[]rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "saconfig",
				Namespace: "default",
			},
		}, rbacv1.RoleRef{
			Kind: roleKind,
			Name: "myrole",
		},
	)

	list[1] = newRoleBinding("test2", "default",
		[]rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "saconfig",
				Namespace: "default",
			},
		}, rbacv1.RoleRef{
			Kind: clusterroleKind,
			Name: "myclusterrole",
		},
	)

	sa := authenticationv1.UserInfo{
		Username: "system:serviceaccount:default:saconfig",
	}

	expectedrole := []string{list[0].Namespace + ":" + "myrole"}
	expectedClusterRole := []string{"myclusterrole"}
	roles, clusterroles := getRoleRefByRoleBindings(list, sa)
	assert.DeepEqual(t, roles, expectedrole)
	assert.DeepEqual(t, clusterroles, expectedClusterRole)
}

func newClusterRoleBinding(name, ns string, subjects []rbacv1.Subject, roles rbacv1.RoleRef) *rbacv1.ClusterRoleBinding {
	rb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Subjects:   subjects,
		RoleRef:    roles,
	}

	rb.Kind = "ClusterRoleBinding"
	rb.APIVersion = "rbac.authorization.k8s.io/v1"
	return rb
}

func Test_getRoleRefByClusterRoleBindings(t *testing.T) {

	list := make([]*rbacv1.ClusterRoleBinding, 2)

	list[0] = newClusterRoleBinding("test1", "mynamespace",
		[]rbacv1.Subject{
			{
				Kind: "User",
				Name: "kube-scheduler",
			},
		}, rbacv1.RoleRef{
			Kind: clusterroleKind,
			Name: "fakeclusterrole",
		},
	)

	list[1] = newClusterRoleBinding("test2", "mynamespace",
		[]rbacv1.Subject{
			{
				Kind: "Group",
				Name: "system:masters",
			},
		}, rbacv1.RoleRef{
			Kind: clusterroleKind,
			Name: "myclusterrole",
		},
	)

	group := authenticationv1.UserInfo{
		Username: "kubernetes-admin",
		Groups:   []string{"system:masters", "system:authenticated"},
	}

	user := authenticationv1.UserInfo{
		Username: "system:kube-scheduler",
		Groups:   []string{"system:authenticated"},
	}

	clusterroles := getRoleRefByClusterRoleBindings(list, group)
	assert.DeepEqual(t, clusterroles, []string{"myclusterrole"})

	clusterroles = getRoleRefByClusterRoleBindings(list, user)
	assert.Equal(t, len(clusterroles), 0)
}
