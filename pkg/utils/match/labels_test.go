package match

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckSelector(t *testing.T) {
	type args struct {
		expected *metav1.LabelSelector
		actual   map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{{
		args: args{
			expected: &metav1.LabelSelector{},
			actual:   map[string]string{},
		},
		want: true,
	}, {
		args: args{
			expected: &metav1.LabelSelector{},
			actual: map[string]string{
				"foo": "bar",
			},
		},
		want: true,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test.io/*": "bar"},
			},
			actual: map[string]string{
				"foo": "bar",
			},
		},
		want: false,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"scale.test.io/*": "bar"},
			},
			actual: map[string]string{
				"foo": "bar",
			},
		},
		want: false,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test.io/*": "bar"},
			},
			actual: map[string]string{
				"test.io/scale":      "foo",
				"test.io/functional": "bar",
			},
		},
		want: true,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test.io/*": "*"},
			},
			actual: map[string]string{
				"test.io/scale":      "foo",
				"test.io/functional": "bar",
			},
		},
		want: true,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test.io/*": "a*"},
			},
			actual: map[string]string{
				"test.io/scale":      "foo",
				"test.io/functional": "bar",
			},
		},
		want: false,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test.io/scale": "f??"},
			},
			actual: map[string]string{
				"test.io/scale":      "foo",
				"test.io/functional": "bar",
			},
		},
		want: true,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"*": "*"},
			},
			actual: map[string]string{
				"test.io/scale":      "foo",
				"test.io/functional": "bar",
			},
		},
		want: true,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test.io/functional": "foo"},
			},
			actual: map[string]string{
				"test.io/scale":      "foo",
				"test.io/functional": "bar",
			},
		},
		want: false,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"*": "*"},
			},
			actual: map[string]string{},
		},
		want: false,
	}, {
		args: args{
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{"abc/def/ghi": "*"},
			},
			actual: map[string]string{},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckSelector(tt.args.expected, tt.args.actual)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckSelector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CheckSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}
