package time

import (
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeLib(t *testing.T) {
	env, err := cel.NewEnv(Lib())
	require.NoError(t, err)

	tests := []struct {
		name     string
		expr     string
		expected interface{}
	}{
		{
			name:     "time_now returns RFC3339 string",
			expr:     `time_now()`,
			expected: types.String(time.Now().Format(time.RFC3339)),
		},
		{
			name:     "time_now_utc returns UTC RFC3339 string",
			expr:     `time_now_utc()`,
			expected: types.String(time.Now().UTC().Format(time.RFC3339)),
		},
		{
			name:     "time_parse with epoch timestamp",
			expr:     `time_parse("0", "1672531200")`,
			expected: types.String(time.Unix(1672531200, 0).UTC().Format(time.RFC3339)),
		},
		{
			name:     "time_parse with RFC3339 layout",
			expr:     `time_parse("2006-01-02T15:04:05Z07:00", "2024-01-15T10:30:00Z")`,
			expected: types.String("2024-01-15T10:30:00Z"),
		},
		{
			name:     "time_add adds duration to timestamp",
			expr:     `time_add("2024-01-15T10:30:00Z", "1h30m")`,
			expected: types.String("2024-01-15T12:00:00Z"),
		},
		{
			name:     "time_diff calculates duration between timestamps",
			expr:     `time_diff("2024-01-15T10:00:00Z", "2024-01-15T10:30:00Z")`,
			expected: types.String("30m0s"),
		},
		{
			name:     "time_before returns true when t1 < t2",
			expr:     `time_before("2024-01-15T10:00:00Z", "2024-01-15T10:30:00Z")`,
			expected: types.Bool(true),
		},
		{
			name:     "time_before returns false when t1 > t2",
			expr:     `time_before("2024-01-15T10:30:00Z", "2024-01-15T10:00:00Z")`,
			expected: types.Bool(false),
		},
		{
			name:     "time_after returns true when t1 > t2",
			expr:     `time_after("2024-01-15T10:30:00Z", "2024-01-15T10:00:00Z")`,
			expected: types.Bool(true),
		},
		{
			name:     "time_after returns false when t1 < t2",
			expr:     `time_after("2024-01-15T10:00:00Z", "2024-01-15T10:30:00Z")`,
			expected: types.Bool(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, issues := env.Compile(tt.expr)
			if issues != nil {
				require.NoError(t, issues.Err())
			}
			
			program, err := env.Program(ast)
			require.NoError(t, err)
			
			out, _, err := program.Eval(map[string]interface{}{})
			require.NoError(t, err)
			
			assert.Equal(t, tt.expected, out)
		})
	}
}