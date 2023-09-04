package test

import (
	"errors"
	"reflect"
	"testing"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
)

func TestTestCases_Errors(t *testing.T) {
	tests := []struct {
		name string
		tc   TestCases
		want []error
	}{{
		name: "nil",
		tc:   nil,
		want: nil,
	}, {
		name: "empty",
		tc:   []TestCase{},
		want: nil,
	}, {
		name: "no error",
		tc:   TestCases([]TestCase{{}}),
		want: nil,
	}, {
		name: "one error",
		tc: []TestCase{{
			Err: errors.New("error 1"),
		}},
		want: []error{
			errors.New("error 1"),
		},
	}, {
		name: "two errors",
		tc: []TestCase{{
			Err: errors.New("error 1"),
		}, {
			Err: errors.New("error 2"),
		}},
		want: []error{
			errors.New("error 1"),
			errors.New("error 2"),
		},
	}, {
		name: "mixed",
		tc: []TestCase{{
			Err: errors.New("error 1"),
		}, {}, {
			Err: errors.New("error 2"),
		}, {}},
		want: []error{
			errors.New("error 1"),
			errors.New("error 2"),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tc.Errors(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TestCases.Errors() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
		dirPath:  "../../testdata/tests/invalid",
		fileName: "kyverno-test.yaml",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "invalid dir",
		dirPath:  "../../testdata/tests",
		fileName: "kyverno-test-invalid.yaml",
		want: []TestCase{{
			Path: "../../testdata/tests/test-invalid/kyverno-test-invalid.yaml",
			Err:  errors.New("error unmarshaling JSON: while decoding JSON: json: unknown field \"foo\""),
		}},
		wantErr: false,
	}, {
		name:     "ok",
		dirPath:  "../../testdata/tests/test-1",
		fileName: "kyverno-test.yaml",
		want: []TestCase{{
			Path: "../../testdata/tests/test-1/kyverno-test.yaml",
			Test: &api.Test{
				Name:      "test-registry",
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []api.TestResults{{
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
		dirPath:  "../../testdata/tests/test-2",
		fileName: "kyverno-test.yaml",
		want: []TestCase{{
			Path: "../../testdata/tests/test-2/kyverno-test.yaml",
			Test: &api.Test{
				Name:      "add-quota",
				Policies:  []string{"policy.yaml"},
				Resources: []string{"resource.yaml"},
				Results: []api.TestResults{{
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
		dirPath:  "../../testdata/tests",
		fileName: "kyverno-test.yaml",
		want: []TestCase{{
			Path: "../../testdata/tests/test-1/kyverno-test.yaml",
			Test: &api.Test{
				Name:      "test-registry",
				Policies:  []string{"image-example.yaml"},
				Resources: []string{"resources.yaml"},
				Results: []api.TestResults{{
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
			Path: "../../testdata/tests/test-2/kyverno-test.yaml",
			Test: &api.Test{
				Name:      "add-quota",
				Policies:  []string{"policy.yaml"},
				Resources: []string{"resource.yaml"},
				Results: []api.TestResults{{
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
	},
	// TODO: Add test cases.
	}
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
