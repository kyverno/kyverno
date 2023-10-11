package test

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
)

func TestTestCase_Dir(t *testing.T) {
	type fields struct {
	}
	tests := []struct {
		name string
		Path string
		Fs   billy.Filesystem
		Test *v1alpha1.Test
		Err  error
		want string
	}{{
		name: "empty",
		want: ".",
	}, {
		name: "relative",
		Path: "foo/bar/baz.yaml",
		want: "foo/bar",
	}, {
		name: "absolute",
		Path: "/foo/bar/baz.yaml",
		want: "/foo/bar",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := TestCase{
				Path: tt.Path,
				Fs:   tt.Fs,
				Test: tt.Test,
				Err:  tt.Err,
			}
			if got := tc.Dir(); got != tt.want {
				t.Errorf("TestCase.Dir() = %v, want %v", got, tt.want)
			}
		})
	}
}
