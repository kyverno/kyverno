package userinfo

import (
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name         string
		fs           billy.Filesystem
		path         string
		resourcePath string
		want         *kyvernov1beta1.RequestInfo
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
		want: &kyvernov1beta1.RequestInfo{
			ClusterRoles: []string{"cluster-admin"},
			AdmissionUserInfo: authenticationv1.UserInfo{
				Username: "molybdenum@somecorp.com",
			},
		},
		wantErr: false,
	},
	}
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
