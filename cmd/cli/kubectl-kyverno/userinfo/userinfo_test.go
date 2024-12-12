package userinfo

import (
	"os"
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	authenticationv1 "k8s.io/api/authentication/v1"
)

func TestLoad(t *testing.T) {
	fs := func(path string) billy.Filesystem {
		t.Helper()
		f := memfs.New()
		file, err := f.Create("valid.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}
		return f
	}
	tests := []struct {
		name         string
		fs           billy.Filesystem
		path         string
		resourcePath string
		want         *v1alpha1.UserInfo
		wantErr      bool
	}{{
		name:         "empty",
		fs:           nil,
		path:         "",
		resourcePath: "",
		want:         nil,
		wantErr:      true,
	}, {
		name:         "invalid",
		fs:           nil,
		path:         "../_testdata/user-infos/invalid.yaml",
		resourcePath: "",
		want:         nil,
		wantErr:      true,
	}, {
		name:         "valid",
		fs:           nil,
		path:         "../_testdata/user-infos/valid.yaml",
		resourcePath: "",
		want: &v1alpha1.UserInfo{
			RequestInfo: kyvernov1beta1.RequestInfo{
				ClusterRoles: []string{"cluster-admin"},
				AdmissionUserInfo: authenticationv1.UserInfo{
					Username: "molybdenum@somecorp.com",
				},
			},
		},
		wantErr: false,
	}, {
		name:         "empty (billy)",
		fs:           fs("../_testdata/user-infos/valid.yaml"),
		path:         "",
		resourcePath: "",
		want:         nil,
		wantErr:      true,
	}, {
		name:         "invalid (billy)",
		fs:           fs("../_testdata/user-infos/valid.yaml"),
		path:         "invalid.yaml",
		resourcePath: "",
		want:         nil,
		wantErr:      true,
	}, {
		name:         "valid (billy)",
		fs:           fs("../_testdata/user-infos/valid.yaml"),
		path:         "valid.yaml",
		resourcePath: "",
		want: &v1alpha1.UserInfo{
			RequestInfo: kyvernov1beta1.RequestInfo{
				ClusterRoles: []string{"cluster-admin"},
				AdmissionUserInfo: authenticationv1.UserInfo{
					Username: "molybdenum@somecorp.com",
				},
			},
		},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(tt.fs, tt.path, tt.resourcePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Load() = %v, want %v", got, tt.want)
			}
		})
	}
}
