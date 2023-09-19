package experimental

import "testing"

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "enabled",
			env: map[string]string{
				ExperimentalEnv: "true",
			},
			want: true,
		},
		{
			name: "enabled",
			env: map[string]string{
				ExperimentalEnv: "1",
			},
			want: true,
		},
		{
			name: "enabled",
			env: map[string]string{
				ExperimentalEnv: "t",
			},
			want: true,
		},
		{
			name: "disabled",
			env: map[string]string{
				ExperimentalEnv: "false",
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
				ExperimentalEnv: "maybe",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
