package d4f

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_breaker_Do(t *testing.T) {
	type args struct {
		inner func(context.Context) error
	}
	tests := []struct {
		name    string
		subject *breaker
		args    args
		wantErr bool
	}{{
		name:    "empty",
		subject: NewBreaker("", nil),
		wantErr: false,
	}, {
		name:    "no error",
		subject: NewBreaker("", nil),
		args: args{
			inner: func(context.Context) error {
				return nil
			},
		},
		wantErr: false,
	}, {
		name:    "with error",
		subject: NewBreaker("", nil),
		args: args{
			inner: func(context.Context) error {
				return errors.New("foo")
			},
		},
		wantErr: true,
	}, {
		name: "with break",
		subject: NewBreaker("", func(context.Context) bool {
			return true
		}),
		args: args{
			inner: func(context.Context) error {
				return errors.New("foo")
			},
		},
		wantErr: false,
	}, {
		name: "with metrics",
		subject: &breaker{
			open: func(context.Context) bool {
				return true
			},
		},
		args: args{
			inner: func(context.Context) error {
				return errors.New("foo")
			},
		},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.subject.Do(context.TODO(), tt.args.inner)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
