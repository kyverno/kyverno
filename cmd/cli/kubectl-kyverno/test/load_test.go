package test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/openreports"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			Err:  fmt.Errorf("error unmarshaling JSON: while decoding JSON: json: unknown field \"foo\""),
		}},
		wantErr: false,
	}, {
		name:     "ok",
		dirPath:  "../_testdata/tests/test-1",
		fileName: "kyverno-test.yaml",
		want: []TestCase{{
			Path: "../_testdata/tests/test-1/kyverno-test.yaml",
			Test: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-registry",
				},
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []v1alpha1.TestResult{{
					TestResultBase: v1alpha1.TestResultBase{
						Kind:   "Pod",
						Policy: "images",
						Result: openreports.StatusPass,
						Rule:   "only-allow-trusted-images",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{
							"test-pod-with-non-root-user-image",
							"test-pod-with-trusted-registry",
						},
					},
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
			Test: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "add-quota",
				},
				Policies:  []string{"policy.yaml"},
				Resources: []string{"resource.yaml"},
				Results: []v1alpha1.TestResult{{
					TestResultBase: v1alpha1.TestResultBase{
						Kind:              "Namespace",
						Policy:            "add-ns-quota",
						Result:            openreports.StatusPass,
						Rule:              "generate-limitrange",
						GeneratedResource: "generatedLimitRange.yaml",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{"hello-world-namespace"},
					},
				}, {
					TestResultBase: v1alpha1.TestResultBase{
						Kind:              "Namespace",
						Policy:            "add-ns-quota",
						Result:            openreports.StatusPass,
						Rule:              "generate-resourcequota",
						GeneratedResource: "generatedResourceQuota.yaml",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{"hello-world-namespace"},
					},
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
			Test: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-registry",
				},
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []v1alpha1.TestResult{{
					TestResultBase: v1alpha1.TestResultBase{
						Kind:   "Pod",
						Policy: "images",
						Result: openreports.StatusPass,
						Rule:   "only-allow-trusted-images",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{
							"test-pod-with-non-root-user-image",
							"test-pod-with-trusted-registry",
						},
					},
				}},
			},
		}, {
			Path: "../_testdata/tests/test-2/kyverno-test.yaml",
			Test: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "add-quota",
				},
				Policies:  []string{"policy.yaml"},
				Resources: []string{"resource.yaml"},
				Results: []v1alpha1.TestResult{{
					TestResultBase: v1alpha1.TestResultBase{
						Kind:              "Namespace",
						Policy:            "add-ns-quota",
						Result:            openreports.StatusPass,
						Rule:              "generate-limitrange",
						GeneratedResource: "generatedLimitRange.yaml",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{"hello-world-namespace"},
					},
				}, {
					TestResultBase: v1alpha1.TestResultBase{
						Kind:              "Namespace",
						Policy:            "add-ns-quota",
						Result:            openreports.StatusPass,
						Rule:              "generate-resourcequota",
						GeneratedResource: "generatedResourceQuota.yaml",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{"hello-world-namespace"},
					},
				}},
			},
		}},
		wantErr: false,
	}, {
		name:     "several-tests-in-one-yaml",
		dirPath:  "../_testdata/tests/test-3",
		fileName: "kyverno-test.yaml",
		want: []TestCase{{
			Path: "../_testdata/tests/test-3/kyverno-test.yaml",
			Test: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-something-1",
				},
				Policies:  []string{"policy-1.yaml"},
				Resources: []string{"resources-1.yaml"},
				Results: []v1alpha1.TestResult{{
					TestResultBase: v1alpha1.TestResultBase{
						Kind:   "Deployment",
						Policy: "policy-1",
						Result: openreports.StatusPass,
						Rule:   "rule-1",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{
							"test-1",
						},
					},
				}},
			},
		}, {
			Path: "../_testdata/tests/test-3/kyverno-test.yaml",
			Test: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-something-2",
				},
				Policies:  []string{"policy-2.yaml"},
				Resources: []string{"resources-2.yaml"},
				Results: []v1alpha1.TestResult{{
					TestResultBase: v1alpha1.TestResultBase{
						Kind:   "Pod",
						Policy: "policy-2",
						Result: openreports.StatusSkip,
						Rule:   "rule-2",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{
							"test-2",
						},
					},
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
			for i := range tt.want {
				if tt.want[i].Err != nil {
					assert.Equal(t, tt.want[i].Err.Error(), got[i].Err.Error())
				} else if !reflect.DeepEqual(got[i], tt.want[i]) {
					t.Errorf("LoadTests() = %v, want %v", got, tt.want)
				}
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
			Test: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-registry",
				},
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []v1alpha1.TestResult{{
					TestResultBase: v1alpha1.TestResultBase{
						Kind:   "Pod",
						Policy: "images",
						Result: openreports.StatusPass,
						Rule:   "only-allow-trusted-images",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{
							"test-pod-with-non-root-user-image",
							"test-pod-with-trusted-registry",
						},
					},
				}},
			},
		},
	}, {
		name: "ok (billy)",
		path: "kyverno-test.yaml",
		want: TestCase{
			Path: "kyverno-test.yaml",
			Test: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-registry",
				},
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []v1alpha1.TestResult{{
					TestResultBase: v1alpha1.TestResultBase{
						Kind:   "Pod",
						Policy: "images",
						Result: openreports.StatusPass,
						Rule:   "only-allow-trusted-images",
					},
					TestResultData: v1alpha1.TestResultData{
						Resources: []string{
							"test-pod-with-non-root-user-image",
							"test-pod-with-trusted-registry",
						},
					},
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
	}, {
		name:    "deprecated schema - missing apiVersion and kind",
		path:    "../_testdata/tests/test-deprecated/kyverno-test.yaml",
		want:    TestCase{Path: "../_testdata/tests/test-deprecated/kyverno-test.yaml"},
		wantErr: true,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCases := LoadTest(tt.fs, tt.path)
			if len(testCases) != 1 {
				t.Fatalf("LoadTest() = %d test cases, want 1", len(testCases))
			}
			got := testCases[0]
			if (got.Err != nil) != tt.wantErr {
				t.Errorf("LoadTest() error = %v, wantErr %v", got.Err, tt.wantErr)
				return
			}
			got.Err = nil
			tt.want.Fs = nil
			got.Fs = nil
			assert.DeepEqual(t, tt.want, got)
		})
	}
}
