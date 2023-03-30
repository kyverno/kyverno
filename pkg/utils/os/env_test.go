package os

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvWithFallback(t *testing.T) {
	type args struct {
		name     string
		fallback string
	}
	tests := []struct {
		name  string
		setup map[string]string
		args  args
		want  string
	}{
		{
			name: "fallback",
			args: args{
				"xxx",
				"yyy",
			},
			want: "yyy",
		},
		{
			name: "no fallback",
			setup: map[string]string{
				"xxx": "zzz",
			},
			args: args{
				"xxx",
				"yyy",
			},
			want: "zzz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.setup {
				t.Setenv(k, v)
			}
			assert.Equal(t, tt.want, GetEnvWithFallback(tt.args.name, tt.args.fallback))
		})
	}
}
