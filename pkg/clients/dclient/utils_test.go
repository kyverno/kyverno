package dclient

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_isServerCurrentlyUnableToHandleRequest(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			err: nil,
		},
		want: false,
	}, {
		args: args{
			err: errors.New("another error"),
		},
		want: false,
	}, {
		args: args{
			err: errors.New("the server is currently unable to handle the request"),
		},
		want: true,
	}, {
		args: args{
			err: errors.New("a prefix : the server is currently unable to handle the request"),
		},
		want: true,
	}, {
		args: args{
			err: errors.New("the server is currently unable to handle the request - a suffix"),
		},
		want: true,
	}, {
		args: args{
			err: errors.New("a prefix : the server is currently unable to handle the request - a suffix"),
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isServerCurrentlyUnableToHandleRequest(tt.args.err); got != tt.want {
				t.Errorf("isServerCurrentlyUnableToHandleRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isMetricsServerUnavailable(t *testing.T) {
	type args struct {
		gv  schema.GroupVersion
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			gv:  schema.GroupVersion{Group: "core", Version: "v1"},
			err: nil,
		},
		want: false,
	}, {
		args: args{
			gv:  schema.GroupVersion{Group: "core", Version: "v1"},
			err: errors.New("the server is currently unable to handle the request"),
		},
		want: false,
	}, {
		args: args{
			gv:  schema.GroupVersion{Group: "metrics.k8s.io", Version: "v1"},
			err: errors.New("the server is currently unable to handle the request"),
		},
		want: true,
	}, {
		args: args{
			gv:  schema.GroupVersion{Group: "custom.metrics.k8s.io", Version: "v1"},
			err: errors.New("the server is currently unable to handle the request"),
		},
		want: true,
	}, {
		args: args{
			gv:  schema.GroupVersion{Group: "external.metrics.k8s.io", Version: "v1"},
			err: errors.New("the server is currently unable to handle the request"),
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMetricsServerUnavailable(tt.args.gv, tt.args.err); got != tt.want {
				t.Errorf("isMetricsServerUnavailable() = %v, want %v", got, tt.want)
			}
		})
	}
}
