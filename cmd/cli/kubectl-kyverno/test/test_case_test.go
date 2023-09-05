package test

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
)

func TestTestCase_Dir(t *testing.T) {
	type fields struct {
		Path string
		Fs   billy.Filesystem
		Test *api.Test
		Err  error
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := TestCase{
				Path: tt.fields.Path,
				Fs:   tt.fields.Fs,
				Test: tt.fields.Test,
				Err:  tt.fields.Err,
			}
			if got := tc.Dir(); got != tt.want {
				t.Errorf("TestCase.Dir() = %v, want %v", got, tt.want)
			}
		})
	}
}
