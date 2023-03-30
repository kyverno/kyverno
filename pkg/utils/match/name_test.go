package match

import "testing"

func TestCheckName(t *testing.T) {
	type args struct {
		expected string
		actual   string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{},
		want: true,
	}, {
		args: args{
			expected: "",
			actual:   "foo",
		},
		want: false,
	}, {
		args: args{
			expected: "*",
			actual:   "foo",
		},
		want: true,
	}, {
		args: args{
			expected: "foo",
			actual:   "foo",
		},
		want: true,
	}, {
		args: args{
			expected: "bar",
			actual:   "foo",
		},
		want: false,
	}, {
		args: args{
			expected: "f?o",
			actual:   "foo",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckName(tt.args.expected, tt.args.actual); got != tt.want {
				t.Errorf("CheckName() = %v, want %v", got, tt.want)
			}
		})
	}
}
