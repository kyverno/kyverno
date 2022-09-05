package os

import (
	"os"
	"testing"
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
		os.Clearenv()
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.setup {
				os.Setenv(k, v)
			}
			if got := GetEnvWithFallback(tt.args.name, tt.args.fallback); got != tt.want {
				t.Errorf("GetEnvWithFallback() = %v, want %v", got, tt.want)
			}
		})
	}
}
