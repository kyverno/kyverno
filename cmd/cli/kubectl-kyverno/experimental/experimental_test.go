package experimental

import "testing"

func TestIsExperimentalEnabled(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "enabled",
			env: map[string]string{
				experimentalEnv: "true",
			},
			want: true,
		},
		{
			name: "enabled",
			env: map[string]string{
				experimentalEnv: "1",
			},
			want: true,
		},
		{
			name: "enabled",
			env: map[string]string{
				experimentalEnv: "t",
			},
			want: true,
		},
		{
			name: "disabled",
			env: map[string]string{
				experimentalEnv: "false",
			},
			want: false,
		},
		{
			name: "not specified",
			env:  map[string]string{},
			want: false,
		},
		{
			name: "bad format",
			env: map[string]string{
				experimentalEnv: "maybe",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := IsExperimentalEnabled(); got != tt.want {
				t.Errorf("IsExperimentalEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
