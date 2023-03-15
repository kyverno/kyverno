package config

import (
	"reflect"
	"testing"
)

func Test_parseRbac(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{{
		args: args{""},
		want: nil,
	}, {
		args: args{"abc"},
		want: []string{"abc"},
	}, {
		args: args{" abc "},
		want: []string{"abc"},
	}, {
		args: args{"abc,def"},
		want: []string{"abc", "def"},
	}, {
		args: args{"abc,,,def,"},
		want: []string{"abc", "def"},
	}, {
		args: args{"abc, def"},
		want: []string{"abc", "def"},
	}, {
		args: args{"abc ,def "},
		want: []string{"abc", "def"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRbac(tt.args.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRbac() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseKinds(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name string
		args args
		want []filter
	}{{
		args: args{""},
		want: []filter{},
	}, {
		args: args{"[]"},
		// TODO: this looks strange
		want: []filter{
			{},
		},
	}, {
		args: args{"[*]"},
		want: []filter{
			{"*", "", ""},
		},
	}, {
		args: args{"[Node]"},
		want: []filter{
			{"Node", "", ""},
		},
	}, {
		args: args{"[Node,*,*]"},
		want: []filter{
			{"Node", "*", "*"},
		},
	}, {
		args: args{"[Pod,default,nginx]"},
		want: []filter{
			{"Pod", "default", "nginx"},
		},
	}, {
		args: args{"[Pod,*,nginx]"},
		want: []filter{
			{"Pod", "*", "nginx"},
		},
	}, {
		args: args{"[Pod,*]"},
		want: []filter{
			{"Pod", "*", ""},
		},
	}, {
		args: args{"[Pod,default,nginx][Pod,kube-system,api-server]"},
		want: []filter{
			{"Pod", "default", "nginx"},
			{"Pod", "kube-system", "api-server"},
		},
	}, {
		args: args{"[Pod,default,nginx],[Pod,kube-system,api-server]"},
		want: []filter{
			{"Pod", "default", "nginx"},
			{"Pod", "kube-system", "api-server"},
		},
	}, {
		args: args{"[Pod,default,nginx] [Pod,kube-system,api-server]"},
		want: []filter{
			{"Pod", "default", "nginx"},
			{"Pod", "kube-system", "api-server"},
		},
	}, {
		args: args{"[Pod,default,nginx]Pod,kube-system,api-server[Pod,kube-system,api-server]"},
		want: []filter{
			{"Pod", "default", "nginx"},
			{"Pod", "kube-system", "api-server"},
		},
	}, {
		args: args{"[Pod,default,nginx,unexpected]"},
		want: []filter{
			{},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseKinds(tt.args.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseKinds() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseIncludeExcludeNamespacesFromNamespacesConfig(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name    string
		args    args
		want    namespacesConfig
		wantErr bool
	}{{
		args:    args{""},
		wantErr: true,
	}, {
		args: args{"null"},
	}, {
		args: args{"{}"},
	}, {
		args:    args{`{"include": "aaa"}`},
		wantErr: true,
	}, {
		args: args{`{"include": ["aaa", "bbb"]}`},
		want: namespacesConfig{
			IncludeNamespaces: []string{"aaa", "bbb"},
		},
	}, {
		args: args{`{"exclude": ["aaa", "bbb"]}`},
		want: namespacesConfig{
			ExcludeNamespaces: []string{"aaa", "bbb"},
		},
	}, {
		args: args{`{"include": ["aaa", "bbb"], "exclude": ["aaa", "bbb"]}`},
		want: namespacesConfig{
			IncludeNamespaces: []string{"aaa", "bbb"},
			ExcludeNamespaces: []string{"aaa", "bbb"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIncludeExcludeNamespacesFromNamespacesConfig(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIncludeExcludeNamespacesFromNamespacesConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIncludeExcludeNamespacesFromNamespacesConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
