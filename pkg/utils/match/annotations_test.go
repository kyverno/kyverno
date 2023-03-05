package match

import "testing"

func TestCheckAnnotations(t *testing.T) {
	type args struct {
		expected map[string]string
		actual   map[string]string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			expected: map[string]string{},
			actual:   map[string]string{},
		},
		want: true,
	}, {
		args: args{
			expected: map[string]string{
				"test/*": "*",
			},
			actual: map[string]string{},
		},
		want: false,
	}, {
		args: args{
			expected: map[string]string{
				"test/*": "*",
			},
			actual: map[string]string{
				"tes1/test": "*",
			},
		},
		want: false,
	}, {
		args: args{
			expected: map[string]string{
				"test/*": "*",
			},
			actual: map[string]string{
				"test/test": "*",
			},
		},
		want: true,
	}, {
		args: args{
			expected: map[string]string{
				"test/*": "*",
			},
			actual: map[string]string{
				"test/bar": "foo",
			},
		},
		want: true,
	}, {
		args: args{
			expected: map[string]string{
				"test/b*": "*",
			},
			actual: map[string]string{
				"test/bar": "foo",
			},
		},
		want: true,
	}, {
		args: args{
			expected: map[string]string{
				"test/b*": "*",
				"test2/*": "*",
			},
			actual: map[string]string{
				"test/bar": "foo",
			},
		},
		want: false,
	}, {
		args: args{
			expected: map[string]string{
				"test/b*": "*",
				"test2/*": "*",
			},
			actual: map[string]string{
				"test/bar":  "foo",
				"test2/123": "bar",
			},
		},
		want: true,
	}, {
		args: args{
			expected: map[string]string{
				"test/b*": "*",
				"test2/*": "*",
			},
			actual: map[string]string{
				"test/bar":  "foo",
				"test2/123": "bar",
				"test3/123": "bar2",
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckAnnotations(tt.args.expected, tt.args.actual); got != tt.want {
				t.Errorf("CheckAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}
