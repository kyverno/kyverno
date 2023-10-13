package config

import (
	"errors"
	"reflect"
	"testing"
)

func Test_parseExclusions(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name           string
		args           args
		wantExclusions []string
		wantInclusions []string
	}{{
		args:           args{""},
		wantExclusions: nil,
	}, {
		args:           args{"abc"},
		wantExclusions: []string{"abc"},
	}, {
		args:           args{" abc "},
		wantExclusions: []string{"abc"},
	}, {
		args:           args{"abc,def"},
		wantExclusions: []string{"abc", "def"},
	}, {
		args:           args{"abc,,,def,"},
		wantExclusions: []string{"abc", "def"},
	}, {
		args:           args{"abc, def"},
		wantExclusions: []string{"abc", "def"},
	}, {
		args:           args{"abc ,def "},
		wantExclusions: []string{"abc", "def"},
	}, {
		args:           args{"abc,!def"},
		wantExclusions: []string{"abc"},
		wantInclusions: []string{"def"},
	}, {
		args:           args{"!def,abc"},
		wantExclusions: []string{"abc"},
		wantInclusions: []string{"def"},
	}, {
		args:           args{"!,abc"},
		wantExclusions: []string{"abc"},
	}, {
		args:           args{"  ! def ,abc"},
		wantExclusions: []string{"abc"},
		wantInclusions: []string{"def"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExclusions, gotInclusions := parseExclusions(tt.args.in)
			if !reflect.DeepEqual(gotExclusions, tt.wantExclusions) {
				t.Errorf("parseExclusions() exclusions = %v, want %v", gotExclusions, tt.wantExclusions)
			}
			if !reflect.DeepEqual(gotInclusions, tt.wantInclusions) {
				t.Errorf("parseExclusions() inclusions = %v, want %v", gotInclusions, tt.wantInclusions)
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
			{"*", "*", "*", "", "", ""},
		},
	}, {
		args: args{"[*/*]"},
		want: []filter{
			{"*", "*", "*", "*", "", ""},
		},
	}, {
		args: args{"[Pod/*]"},
		want: []filter{
			{"*", "*", "Pod", "*", "", ""},
		},
	}, {
		args: args{"[v1/Pod/*]"},
		want: []filter{
			{"*", "v1", "Pod", "*", "", ""},
		},
	}, {
		args: args{"[v1/Pod]"},
		want: []filter{
			{"*", "v1", "Pod", "", "", ""},
		},
	}, {
		args: args{"[Node]"},
		want: []filter{
			{"*", "*", "Node", "", "", ""},
		},
	}, {
		args: args{"[Node,*,*]"},
		want: []filter{
			{"*", "*", "Node", "", "*", "*"},
		},
	}, {
		args: args{"[Pod,default,nginx]"},
		want: []filter{
			{"*", "*", "Pod", "", "default", "nginx"},
		},
	}, {
		args: args{"[Pod,*,nginx]"},
		want: []filter{
			{"*", "*", "Pod", "", "*", "nginx"},
		},
	}, {
		args: args{"[Pod,*]"},
		want: []filter{
			{"*", "*", "Pod", "", "*", ""},
		},
	}, {
		args: args{"[Pod,default,nginx][Pod,kube-system,api-server]"},
		want: []filter{
			{"*", "*", "Pod", "", "default", "nginx"},
			{"*", "*", "Pod", "", "kube-system", "api-server"},
		},
	}, {
		args: args{"[Pod,default,nginx],[Pod,kube-system,api-server]"},
		want: []filter{
			{"*", "*", "Pod", "", "default", "nginx"},
			{"*", "*", "Pod", "", "kube-system", "api-server"},
		},
	}, {
		args: args{"[Pod,default,nginx] [Pod,kube-system,api-server]"},
		want: []filter{
			{"*", "*", "Pod", "", "default", "nginx"},
			{"*", "*", "Pod", "", "kube-system", "api-server"},
		},
	}, {
		args: args{"[Pod,default,nginx]Pod,kube-system,api-server[Pod,kube-system,api-server]"},
		want: []filter{
			{"*", "*", "Pod", "", "default", "nginx"},
			{"*", "*", "Pod", "", "kube-system", "api-server"},
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

func Test_parseWebhookAnnotations(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{{
		args:    args{"hello"},
		wantErr: true,
	}, {
		args:    args{""},
		wantErr: true,
	}, {
		args: args{"null"},
	}, {
		args: args{`{"a": "b"}`},
		want: map[string]string{
			"a": "b",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWebhookAnnotations(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseWebhookAnnotations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseWebhookAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseBucketBoundariesConfig(t *testing.T) {
	var emptyBoundaries []float64

	tests := []struct {
		input         string
		expected      []float64
		expectedError error
	}{
		{"0.005, 0.01, 0.025, 0.05", []float64{0.005, 0.01, 0.025, 0.05}, nil},
		{"0.1, 0.2, 0.3", []float64{0.1, 0.2, 0.3}, nil},
		{"0.1,0.2,0.3", []float64{0.1, 0.2, 0.3}, nil},
		{"", emptyBoundaries, nil},
		{" ", emptyBoundaries, nil},
		{"invalid, 0.01, 0.025, 0.05", nil, errors.New("invalid boundary value 'invalid'")},
		{"0.005, 0.01, , 0.05", nil, errors.New("invalid boundary value ''")},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			boundaries, err := parseBucketBoundariesConfig(test.input)

			if !reflect.DeepEqual(boundaries, test.expected) {
				t.Errorf("Expected boundaries %v, but got %v", test.expected, boundaries)
			}

			if (err == nil && test.expectedError != nil) || (err != nil && err.Error() != test.expectedError.Error()) {
				t.Errorf("Expected error '%v', but got '%v'", test.expectedError, err)
			}
		})
	}
}

func Test_parseMetricExposureConfig(t *testing.T) {
	boolPtr := func(b bool) *bool {
		return &b
	}
	defaultBoundaries := []float64{0.005, 0.01}
	tests := []struct {
		input         string
		expected      map[string]metricExposureConfig
		expectedError bool
	}{
		// Test case 1: Valid JSON with "enabled", "disabledLabelDimensions" and "bucketBoundaries" set
		{
			input: `{
				"key1": {"enabled": true, "disabledLabelDimensions": ["dim1", "dim2"], "bucketBoundaries": []},
				"key2": {"enabled": false, "disabledLabelDimensions": [], "bucketBoundaries": [1.01, 2.5, 5, 10]}
			}`,
			expected: map[string]metricExposureConfig{
				"key1": {Enabled: boolPtr(true), DisabledLabelDimensions: []string{"dim1", "dim2"}, BucketBoundaries: []float64{}},
				"key2": {Enabled: boolPtr(false), DisabledLabelDimensions: []string{}, BucketBoundaries: []float64{1.01, 2.5, 5, 10}},
			},
			expectedError: false,
		},
		// Test case 2: Valid JSON with only "disabledLabelDimensions" set
		{
			input: `{
				"key1": {"disabledLabelDimensions": ["dim1", "dim2"]}
			}`,
			expected: map[string]metricExposureConfig{
				"key1": {Enabled: boolPtr(true), DisabledLabelDimensions: []string{"dim1", "dim2"}, BucketBoundaries: defaultBoundaries},
			},
			expectedError: false,
		},
		// Test case 3: Valid JSON with "enabled" set to false
		{
			input: `{
				"key1": {"enabled": false}
			}`,
			expected: map[string]metricExposureConfig{
				"key1": {Enabled: boolPtr(false), DisabledLabelDimensions: []string{}, BucketBoundaries: defaultBoundaries},
			},
			expectedError: false,
		},
		// Test case 4: Valid JSON with only "bucketBoundaries" set
		{
			input: `{
				"key1": {"bucketBoundaries": []},
				"key2": {"bucketBoundaries": [1.01, 2.5, 5, 10]}
			}`,
			expected: map[string]metricExposureConfig{
				"key1": {Enabled: boolPtr(true), DisabledLabelDimensions: []string{}, BucketBoundaries: []float64{}},
				"key2": {Enabled: boolPtr(true), DisabledLabelDimensions: []string{}, BucketBoundaries: []float64{1.01, 2.5, 5, 10}},
			},
			expectedError: false,
		},
		// Test case 5: Invalid JSON
		{
			input:         "invalid-json",
			expected:      nil,
			expectedError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			configMap, err := parseMetricExposureConfig(test.input, defaultBoundaries)

			if test.expectedError && err == nil {
				t.Error("Expected an error, but got nil")
			}

			if !test.expectedError && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			if !reflect.DeepEqual(configMap, test.expected) {
				t.Errorf("Expected %+v, but got %+v", test.expected, configMap)
			}
		})
	}
}
