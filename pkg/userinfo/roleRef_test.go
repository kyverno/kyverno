package userinfo

import (
	"errors"
	"reflect"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
)

func Test_getRoleRefByRoleBindings(t *testing.T) {
	roleInSameNsImplicit := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "same-ns",
			Namespace: "ns-1",
		},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: "sa",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "role-1",
		},
	}
	roleInSameNsExplicit := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "same-ns",
			Namespace: "ns-1",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "sa",
			Namespace: "ns-1",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "role-1",
		},
	}
	roleInAnotherNs := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "different-ns",
			Namespace: "ns-2",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "sa",
			Namespace: "ns-1",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "role-1",
		},
	}
	clusterRole := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "different-ns",
			Namespace: "ns-2",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "sa",
			Namespace: "ns-1",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "role-1",
		},
	}
	userInfo := authenticationv1.UserInfo{
		Username: "system:serviceaccount:ns-1:sa",
	}
	type args struct {
		roleBindings []*rbacv1.RoleBinding
		userInfo     authenticationv1.UserInfo
	}
	tests := []struct {
		name             string
		args             args
		wantRoles        []string
		wantClusterRoles []string
	}{{
		name: "service account and role binding explicitely in the same namespace",
		args: args{
			roleBindings: []*rbacv1.RoleBinding{
				roleInSameNsExplicit,
			},
			userInfo: userInfo,
		},
		wantRoles: []string{
			"ns-1:role-1",
		},
	}, {
		name: "service account and role binding implicitely in the same namespace",
		args: args{
			roleBindings: []*rbacv1.RoleBinding{
				roleInSameNsImplicit,
			},
			userInfo: userInfo,
		},
		wantRoles: []string{
			"ns-1:role-1",
		},
	}, {
		name: "service account and role binding in the different namespaces",
		args: args{
			roleBindings: []*rbacv1.RoleBinding{
				roleInAnotherNs,
			},
			userInfo: userInfo,
		},
		wantRoles: []string{
			"ns-2:role-1",
		},
	}, {
		name: "cluster role",
		args: args{
			roleBindings: []*rbacv1.RoleBinding{
				clusterRole,
			},
			userInfo: userInfo,
		},
		wantClusterRoles: []string{
			"role-1",
		},
	}, {
		name: "all together",
		args: args{
			roleBindings: []*rbacv1.RoleBinding{
				roleInSameNsExplicit,
				roleInSameNsImplicit,
				roleInAnotherNs,
				clusterRole,
			},
			userInfo: userInfo,
		},
		wantRoles: []string{
			"ns-1:role-1",
			"ns-1:role-1",
			"ns-2:role-1",
		},
		wantClusterRoles: []string{
			"role-1",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRoles, gotClusterRoles := getRoleRefByRoleBindings(tt.args.roleBindings, tt.args.userInfo)
			if !reflect.DeepEqual(gotRoles, tt.wantRoles) {
				t.Errorf("getRoleRefByRoleBindings() gotRoles = %v, want %v", gotRoles, tt.wantRoles)
			}
			if !reflect.DeepEqual(gotClusterRoles, tt.wantClusterRoles) {
				t.Errorf("getRoleRefByRoleBindings() gotClusterRoles = %v, want %v", gotClusterRoles, tt.wantClusterRoles)
			}
		})
	}
}

func Test_matchBindingSubjects(t *testing.T) {
	type args struct {
		subjects  []rbacv1.Subject
		userInfo  authenticationv1.UserInfo
		namespace string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "empty subjects",
		args: args{
			subjects: []rbacv1.Subject{},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:foo:test",
			},
			namespace: "",
		},
		want: false,
	}, {
		name: "nil subjects",
		args: args{
			subjects: nil,
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:foo:test",
			},
			namespace: "",
		},
		want: false,
	}, {
		name: "match service account",
		args: args{
			subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "test",
				Namespace: "foo",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:foo:test",
			},
			namespace: "",
		},
		want: true,
	}, {
		name: "match service account with fallback namespace",
		args: args{
			subjects: []rbacv1.Subject{{
				Kind: rbacv1.ServiceAccountKind,
				Name: "test",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:foo:test",
			},
			namespace: "foo",
		},
		want: true,
	}, {
		name: "don't match service account",
		args: args{
			subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "test",
				Namespace: "foo",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:bar:test",
			},
			namespace: "",
		},
		want: false,
	}, {
		name: "don't match service account with fallback namespace",
		args: args{
			subjects: []rbacv1.Subject{{
				Kind: rbacv1.ServiceAccountKind,
				Name: "test",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:bar:test",
			},
			namespace: "foo",
		},
		want: false,
	}, {
		name: "don't match service account with no namespace and no fallback namespace",
		args: args{
			subjects: []rbacv1.Subject{{
				Kind: rbacv1.ServiceAccountKind,
				Name: "test",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount::test",
			},
			namespace: "",
		},
		want: false,
	}, {
		name: "match user",
		args: args{
			subjects: []rbacv1.Subject{{
				Kind: rbacv1.UserKind,
				Name: "someone@company.org",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "someone@company.org",
			},
			namespace: "",
		},
		want: true,
	}, {
		name: "don't match user",
		args: args{
			subjects: []rbacv1.Subject{{
				Kind: rbacv1.UserKind,
				Name: "someone@company.org",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "someone-else@company.org",
			},
			namespace: "",
		},
		want: false,
	}, {
		name: "match group",
		args: args{
			subjects: []rbacv1.Subject{{
				Kind: rbacv1.GroupKind,
				Name: "admin",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "someone@company.org",
				Groups: []string{
					"user",
					"dev",
					"admin",
				},
			},
			namespace: "",
		},
		want: true,
	}, {
		name: "don't match group",
		args: args{
			subjects: []rbacv1.Subject{{
				Kind: rbacv1.GroupKind,
				Name: "marketing",
			}},
			userInfo: authenticationv1.UserInfo{
				Username: "someone@company.org",
				Groups: []string{
					"user",
					"dev",
					"admin",
				},
			},
			namespace: "",
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchBindingSubjects(tt.args.subjects, tt.args.userInfo, tt.args.namespace); got != tt.want {
				t.Errorf("matchBindingSubjects() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getRoleRefByClusterRoleBindings(t *testing.T) {
	clusterRole1 := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-role",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "sa",
			Namespace: "foo",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "role-1",
		},
	}
	clusterRole2 := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-role",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "sa",
			Namespace: "bar",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "role-2",
		},
	}
	userInfo := authenticationv1.UserInfo{
		Username: "system:serviceaccount:foo:sa",
	}
	type args struct {
		clusterroleBindings []*rbacv1.ClusterRoleBinding
		userInfo            authenticationv1.UserInfo
	}
	tests := []struct {
		name             string
		args             args
		wantClusterRoles []string
	}{{
		name: "match service account",
		args: args{
			clusterroleBindings: []*rbacv1.ClusterRoleBinding{
				clusterRole1,
			},
			userInfo: userInfo,
		},
		wantClusterRoles: []string{
			"role-1",
		},
	}, {
		name: "sa in another namespace",
		args: args{
			clusterroleBindings: []*rbacv1.ClusterRoleBinding{
				clusterRole2,
			},
			userInfo: userInfo,
		},
	}, {
		name: "match service account",
		args: args{
			clusterroleBindings: []*rbacv1.ClusterRoleBinding{
				clusterRole1,
				clusterRole2,
			},
			userInfo: userInfo,
		},
		wantClusterRoles: []string{
			"role-1",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotClusterRoles := getRoleRefByClusterRoleBindings(tt.args.clusterroleBindings, tt.args.userInfo); !reflect.DeepEqual(gotClusterRoles, tt.wantClusterRoles) {
				t.Errorf("getRoleRefByClusterRoleBindings() = %v, want %v", gotClusterRoles, tt.wantClusterRoles)
			}
		})
	}
}

type roleBindingLister struct {
	ret []*rbacv1.RoleBinding
	err error
}

func (l roleBindingLister) List(labels.Selector) ([]*rbacv1.RoleBinding, error) {
	return l.ret, l.err
}

type clusterRoleBindingLister struct {
	ret []*rbacv1.ClusterRoleBinding
	err error
}

func (l clusterRoleBindingLister) List(labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
	return l.ret, l.err
}

func TestGetRoleRef(t *testing.T) {
	type args struct {
		rbLister  RoleBindingLister
		crbLister ClusterRoleBindingLister
		userInfo  authenticationv1.UserInfo
	}
	type want struct {
		roles        []string
		clusterRoles []string
	}
	tests := []struct {
		name    string
		args    args
		want    want
		wantErr bool
	}{
		{
			args: args{
				rbLister: roleBindingLister{
					ret: []*rbacv1.RoleBinding{{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "same-ns",
							Namespace: "ns-1",
						},
						Subjects: []rbacv1.Subject{{
							Kind: "ServiceAccount",
							Name: "sa",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "Role",
							Name: "role-1",
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "same-ns",
							Namespace: "ns-1",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "ns-1",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "Role",
							Name: "role-1",
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "different-ns",
							Namespace: "ns-2",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "ns-1",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "Role",
							Name: "role-1",
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "different-ns",
							Namespace: "ns-2",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "ns-1",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "role-1",
						},
					}},
				},
				crbLister: clusterRoleBindingLister{},
				userInfo: authenticationv1.UserInfo{
					Username: "system:serviceaccount:ns-1:sa",
				},
			},
			want: want{
				roles:        []string{"ns-1:role-1", "ns-2:role-1"},
				clusterRoles: []string{"role-1"},
			},
		}, {
			args: args{
				rbLister: roleBindingLister{
					err: errors.New("error"),
				},
				crbLister: clusterRoleBindingLister{},
				userInfo: authenticationv1.UserInfo{
					Username: "system:serviceaccount:ns-1:sa",
				},
			},
			wantErr: true,
		}, {
			args: args{
				rbLister: roleBindingLister{},
				crbLister: clusterRoleBindingLister{
					err: errors.New("error"),
				},
				userInfo: authenticationv1.UserInfo{
					Username: "system:serviceaccount:ns-1:sa",
				},
			},
			wantErr: true,
		}, {
			args: args{
				rbLister: roleBindingLister{
					err: errors.New("error"),
				},
				crbLister: clusterRoleBindingLister{
					err: errors.New("error"),
				},
				userInfo: authenticationv1.UserInfo{
					Username: "system:serviceaccount:ns-1:sa",
				},
			},
			wantErr: true,
		}, {
			args: args{
				rbLister: roleBindingLister{},
				crbLister: clusterRoleBindingLister{
					ret: []*rbacv1.ClusterRoleBinding{{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-role",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "foo",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "role-1",
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-role",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "bar",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "role-2",
						},
					}},
				},
				userInfo: authenticationv1.UserInfo{
					Username: "system:serviceaccount:foo:sa",
				},
			},
			want: want{
				clusterRoles: []string{"role-1"},
			},
		}, {
			args: args{
				rbLister: roleBindingLister{
					ret: []*rbacv1.RoleBinding{{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "same-ns",
							Namespace: "foo",
						},
						Subjects: []rbacv1.Subject{{
							Kind: "ServiceAccount",
							Name: "sa",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "Role",
							Name: "role-1",
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "same-ns",
							Namespace: "foo",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "foo",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "Role",
							Name: "role-1",
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "different-ns",
							Namespace: "ns-2",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "foo",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "Role",
							Name: "role-1",
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "different-ns",
							Namespace: "ns-2",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "foo",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "role-1",
						},
					}},
				},
				crbLister: clusterRoleBindingLister{
					ret: []*rbacv1.ClusterRoleBinding{{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-role",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "foo",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "role-1",
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-role",
						},
						Subjects: []rbacv1.Subject{{
							Kind:      "ServiceAccount",
							Name:      "sa",
							Namespace: "bar",
						}},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "role-2",
						},
					}},
				},
				userInfo: authenticationv1.UserInfo{
					Username: "system:serviceaccount:foo:sa",
				},
			},
			want: want{
				roles:        []string{"foo:role-1", "ns-2:role-1"},
				clusterRoles: []string{"role-1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roles, clusterRoles, err := GetRoleRef(tt.args.rbLister, tt.args.crbLister, tt.args.userInfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRoleRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(roles, tt.want.roles) {
				t.Errorf("GetRoleRef() roles = %v, want %v", roles, tt.want.roles)
			}
			if !reflect.DeepEqual(clusterRoles, tt.want.clusterRoles) {
				t.Errorf("GetRoleRef() clusterRoles = %v, want %v", clusterRoles, tt.want.clusterRoles)
			}
		})
	}
}
