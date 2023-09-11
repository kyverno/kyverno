package test

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	testapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/test"
)

func TestLoadTests(t *testing.T) {
	tests := []struct {
		name     string
		dirPath  string
		fileName string
		want     TestCases
		wantErr  bool
	}{{
		name:     "empty dir",
		dirPath:  "",
		fileName: "kyverno-test.yaml",
		want:     nil,
		wantErr:  false,
	}, {
		name:     "invalid dir",
		dirPath:  "../_testdata/tests/invalid",
		fileName: "kyverno-test.yaml",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "invalid dir",
		dirPath:  "../_testdata/tests",
		fileName: "kyverno-test-invalid.yaml",
		want: []TestCase{{
			Path: "../_testdata/tests/test-invalid/kyverno-test-invalid.yaml",
			Err:  errors.New("error unmarshaling JSON: while decoding JSON: json: unknown field \"foo\""),
		}},
		wantErr: false,
	}, {
		name:     "ok",
		dirPath:  "../_testdata/tests/test-1",
		fileName: "kyverno-test.yaml",
		want: []TestCase{{
			Path: "../_testdata/tests/test-1/kyverno-test.yaml",
			Test: &testapi.Test{
				Name:      "test-registry",
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []testapi.TestResults{{
					Kind:      "Pod",
					Policy:    "images",
					Resources: []string{"test-pod-with-non-root-user-image"},
					Result:    policyreportv1alpha2.StatusPass,
					Rule:      "only-allow-trusted-images",
				}, {
					Kind:      "Pod",
					Policy:    "images",
					Resources: []string{"test-pod-with-trusted-registry"},
					Result:    policyreportv1alpha2.StatusPass,
					Rule:      "only-allow-trusted-images",
				}},
			},
		}},
		wantErr: false,
	}, {
		name:     "ok",
		dirPath:  "../_testdata/tests/test-2",
		fileName: "kyverno-test.yaml",
		want: []TestCase{{
			Path: "../_testdata/tests/test-2/kyverno-test.yaml",
			Test: &testapi.Test{
				Name:      "add-quota",
				Policies:  []string{"policy.yaml"},
				Resources: []string{"resource.yaml"},
				Results: []testapi.TestResults{{
					Kind:              "Namespace",
					Policy:            "add-ns-quota",
					Resources:         []string{"hello-world-namespace"},
					Result:            policyreportv1alpha2.StatusPass,
					Rule:              "generate-resourcequota",
					GeneratedResource: "generatedResourceQuota.yaml",
				}, {
					Kind:              "Namespace",
					Policy:            "add-ns-quota",
					Resources:         []string{"hello-world-namespace"},
					Result:            policyreportv1alpha2.StatusPass,
					Rule:              "generate-limitrange",
					GeneratedResource: "generatedLimitRange.yaml",
				}},
			},
		}},
		wantErr: false,
	}, {
		name:     "ok",
		dirPath:  "../_testdata/tests",
		fileName: "kyverno-test.yaml",
		want: []TestCase{{
			Path: "../_testdata/tests/test-1/kyverno-test.yaml",
			Test: &testapi.Test{
				Name:      "test-registry",
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []testapi.TestResults{{
					Kind:      "Pod",
					Policy:    "images",
					Resources: []string{"test-pod-with-non-root-user-image"},
					Result:    policyreportv1alpha2.StatusPass,
					Rule:      "only-allow-trusted-images",
				}, {
					Kind:      "Pod",
					Policy:    "images",
					Resources: []string{"test-pod-with-trusted-registry"},
					Result:    policyreportv1alpha2.StatusPass,
					Rule:      "only-allow-trusted-images",
				}},
			},
		}, {
			Path: "../_testdata/tests/test-2/kyverno-test.yaml",
			Test: &testapi.Test{
				Name:      "add-quota",
				Policies:  []string{"policy.yaml"},
				Resources: []string{"resource.yaml"},
				Results: []testapi.TestResults{{
					Kind:              "Namespace",
					Policy:            "add-ns-quota",
					Resources:         []string{"hello-world-namespace"},
					Result:            policyreportv1alpha2.StatusPass,
					Rule:              "generate-resourcequota",
					GeneratedResource: "generatedResourceQuota.yaml",
				}, {
					Kind:              "Namespace",
					Policy:            "add-ns-quota",
					Resources:         []string{"hello-world-namespace"},
					Result:            policyreportv1alpha2.StatusPass,
					Rule:              "generate-limitrange",
					GeneratedResource: "generatedLimitRange.yaml",
				}},
			},
		}},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadTests(tt.dirPath, tt.fileName)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadTests() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadTests() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadTest(t *testing.T) {
	mustReadFile := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	tests := []struct {
		name    string
		fs      billy.Filesystem
		path    string
		want    TestCase
		wantErr bool
	}{{
		name:    "empty",
		path:    "",
		wantErr: true,
	}, {
		name: "ok",
		path: "../_testdata/tests/test-1/kyverno-test.yaml",
		want: TestCase{
			Path: "../_testdata/tests/test-1/kyverno-test.yaml",
			Test: &testapi.Test{
				Name:      "test-registry",
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []testapi.TestResults{{
					Kind:      "Pod",
					Policy:    "images",
					Resources: []string{"test-pod-with-non-root-user-image"},
					Result:    policyreportv1alpha2.StatusPass,
					Rule:      "only-allow-trusted-images",
				}, {
					Kind:      "Pod",
					Policy:    "images",
					Resources: []string{"test-pod-with-trusted-registry"},
					Result:    policyreportv1alpha2.StatusPass,
					Rule:      "only-allow-trusted-images",
				}},
			},
		},
	}, {
		name: "ok (billy)",
		path: "kyverno-test.yaml",
		want: TestCase{
			Path: "kyverno-test.yaml",
			Test: &testapi.Test{
				Name:      "test-registry",
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []testapi.TestResults{{
					Kind:      "Pod",
					Policy:    "images",
					Resources: []string{"test-pod-with-non-root-user-image"},
					Result:    policyreportv1alpha2.StatusPass,
					Rule:      "only-allow-trusted-images",
				}, {
					Kind:      "Pod",
					Policy:    "images",
					Resources: []string{"test-pod-with-trusted-registry"},
					Result:    policyreportv1alpha2.StatusPass,
					Rule:      "only-allow-trusted-images",
				}},
			},
		},
		fs: func() billy.Filesystem {
			f := memfs.New()
			file, err := f.Create("kyverno-test.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := file.Write(mustReadFile("../_testdata/tests/test-1/kyverno-test.yaml")); err != nil {
				t.Fatal(err)
			}
			return f
		}(),
	}, {
		name: "bad file (billy)",
		path: "kyverno-test-bad.yaml",
		fs: func() billy.Filesystem {
			f := memfs.New()
			file, err := f.Create("kyverno-test.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := file.Write(mustReadFile("../_testdata/tests/test-1/kyverno-test.yaml")); err != nil {
				t.Fatal(err)
			}
			return f
		}(),
		want: TestCase{
			Path: "kyverno-test-bad.yaml",
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LoadTest(tt.fs, tt.path)
			if (got.Err != nil) != tt.wantErr {
				t.Errorf("LoadTest() error = %v, wantErr %v", got.Err, tt.wantErr)
				return
			}
			got.Err = nil
			tt.want.Fs = tt.fs
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadTest() = %v, want %v", got, tt.want)
			}
		})
	}
}
