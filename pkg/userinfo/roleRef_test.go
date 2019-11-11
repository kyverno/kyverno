package userinfo

import (
	"flag"
	"reflect"
	"testing"

	"gotest.tools/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_isServiceaccountUserInfo(t *testing.T) {
	tests := []struct {
		username string
		expected bool
	}{
		{
			username: "system:serviceaccount:default:saconfig",
			expected: true,
		},
		{
			username: "serviceaccount:default:saconfig",
			expected: false,
		},
	}

	for _, test := range tests {
		res := isServiceaccountUserInfo(test.username)
		assert.Assert(t, test.expected == res)
	}
}

func Test_matchServiceAccount_subject_variants(t *testing.T) {
	userInfo := authenticationv1.UserInfo{
		Username: "system:serviceaccount:default:saconfig",
	}

	tests := []struct {
		subject  map[string]interface{}
		expected bool
	}{
		{
			subject:  make(map[string]interface{}, 1),
			expected: false,
		},
		{
			subject: map[string]interface{}{
				"kind": "serviceaccount",
			},
			expected: false,
		},
		{
			subject: map[string]interface{}{
				"kind":      "ServiceAccount",
				"Namespace": "testnamespace",
			},
			expected: false,
		},
		{
			subject: map[string]interface{}{
				"kind":      "ServiceAccount",
				"namespace": 1,
			},
			expected: false,
		},
		{
			subject: map[string]interface{}{
				"kind":      "ServiceAccount",
				"namespace": "testnamespace",
				"names":     "",
			},
			expected: false,
		},
		{
			subject: map[string]interface{}{
				"kind":      "ServiceAccount",
				"namespace": "testnamespace",
				"name":      "testname",
			},
			expected: false,
		},
	}

	for _, test := range tests {
		res := matchServiceAccount(test.subject, userInfo)
		assert.Assert(t, test.expected == res)
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

	userContext := map[string]interface{}{
		"kind": "User",
		"name": "system:kube-scheduler",
	}

	groupContext := map[string]interface{}{
		"kind": "Group",
		"name": "system:masters",
	}

	fakegroupContext := map[string]interface{}{
		"kind": "Group",
		"name": "fakeGroup",
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

	sasubject := map[string]interface{}{
		"kind":      "ServiceAccount",
		"namespace": "default",
		"name":      "saconfig",
	}

	groupsubject := map[string]interface{}{
		"kind": "Group",
		"name": "fakeGroup",
	}

	res := matchSubjectsMap(sasubject, sa)
	assert.Assert(t, res)

	res = matchSubjectsMap(groupsubject, group)
	assert.Assert(t, !res)
}

func Test_getRoleRefByRoleBindings(t *testing.T) {
	flag.Parse()
	flag.Set("logtostderr", "true")
	flag.Set("v", "3")
	list := &unstructured.UnstructuredList{
		Object: map[string]interface{}{"kind": "List", "apiVersion": "v1"},
		Items: []unstructured.Unstructured{
			{
				Object: map[string]interface{}{
					"kind":       "RoleBinding",
					"apiVersion": "rbac.authorization.k8s.io/v1",
					"metadata":   map[string]interface{}{"name": "test1"},
					"roleRef": map[string]interface{}{
						"kind":      "role",
						"name":      "myrole",
						"namespace": "mynamespace",
					},
					"subjects": []map[string]interface{}{
						{
							"kind":      "ServiceAccount",
							"name":      "saconfig",
							"namespace": "default",
						},
					},
				},
			},
			{
				Object: map[string]interface{}{
					"kind":       "RoleBinding",
					"apiVersion": "rbac.authorization.k8s.io/v1",
					"metadata":   map[string]interface{}{"name": "test2"},
					"roleRef": map[string]interface{}{
						"kind": "clusterRole",
						"name": "myclusterrole",
					},
					"subjects": []map[string]interface{}{
						{
							"kind":      "ServiceAccount",
							"name":      "saconfig",
							"namespace": "default",
						},
					},
				},
			},
		},
	}

	sa := authenticationv1.UserInfo{
		Username: "system:serviceaccount:default:saconfig",
	}

	expectedrole := []string{"mynamespace:myrole"}
	expectedClusterRole := []string{"myclusterrole"}
	roles, clusterroles, err := getRoleRefByRoleBindings(list, sa)
	assert.Assert(t, err == nil)
	assert.Assert(t, reflect.DeepEqual(roles, expectedrole))
	assert.Assert(t, reflect.DeepEqual(clusterroles, expectedClusterRole))
}

func Test_getRoleRefByClusterRoleBindings(t *testing.T) {
	list := &unstructured.UnstructuredList{
		Object: map[string]interface{}{"kind": "List", "apiVersion": "v1"},
		Items: []unstructured.Unstructured{
			{
				Object: map[string]interface{}{
					"kind":       "ClusterRoleBinding",
					"apiVersion": "rbac.authorization.k8s.io/v1",
					"metadata":   map[string]interface{}{"name": "test-1"},
					"roleRef": map[string]interface{}{
						"kind": "clusterRole",
						"name": "fakeclusterrole",
					},
					"subjects": []map[string]interface{}{
						{
							"kind": "User",
							"name": "kube-scheduler",
						},
					},
				},
			},
			{
				Object: map[string]interface{}{
					"kind":       "ClusterRoleBinding",
					"apiVersion": "rbac.authorization.k8s.io/v1",
					"metadata":   map[string]interface{}{"name": "test-2"},
					"roleRef": map[string]interface{}{
						"kind": "clusterRole",
						"name": "myclusterrole",
					},
					"subjects": []map[string]interface{}{
						{
							"kind": "Group",
							"name": "system:masters",
						},
					},
				},
			},
		},
	}

	group := authenticationv1.UserInfo{
		Username: "kubernetes-admin",
		Groups:   []string{"system:masters", "system:authenticated"},
	}

	user := authenticationv1.UserInfo{
		Username: "system:kube-scheduler",
		Groups:   []string{"system:authenticated"},
	}

	clusterroles, err := getRoleRefByClusterRoleBindings(list, group)
	assert.Assert(t, err == nil)
	assert.Assert(t, reflect.DeepEqual(clusterroles, []string{"myclusterrole"}))

	clusterroles, err = getRoleRefByClusterRoleBindings(list, user)
	assert.Assert(t, err == nil)
	assert.Assert(t, len(clusterroles) == 0)
}
