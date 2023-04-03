package dclient

import (
	"errors"
	"testing"
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
