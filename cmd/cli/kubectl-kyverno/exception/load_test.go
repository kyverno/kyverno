package exception

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Load(t *testing.T) {
	tests := []struct {
		name       string
		policies   string
		wantLoaded int
		wantErr    bool
	}{{
		name:     "not a policy exception",
		policies: "../_testdata/resources/namespace.yaml",
		wantErr:  true,
	}, {
		name:       "policy exception",
		policies:   "../_testdata/exceptions/exception.yaml",
		wantLoaded: 1,
	}, {
		name:     "policy exception and policy",
		policies: "../_testdata/exceptions/exception-and-policy.yaml",
		wantErr:  true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := os.ReadFile(tt.policies)
			require.NoError(t, err)
			require.NoError(t, err)
			if res, err := Load(bytes); (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			} else if len(res) != tt.wantLoaded {
				t.Errorf("Load() loaded amount = %v, wantLoaded %v", len(res), tt.wantLoaded)
			}
		})
	}
}
