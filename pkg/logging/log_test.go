package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected string
		wantErr  bool
	}{
		{"ValidTextFormat", TextFormat, "text", false},
		{"ValidJSONFormat", JSONFormat, "json", false},
		{"ValidECSFormat", EcsFormat, "ecs", false},
		{"InvalidFormat", "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			err := Setup(tt.format, DefaultTime, 0)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			globalLog.Info("test-log")

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			logOutput := buf.String()

			require.NotEmpty(t, logOutput, "Expected non-empty log output")

			if tt.expected == "text" {
				return
			}

			var logData map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(logOutput), &logData))

			if tt.expected == "json" {
				assert.Contains(t, logData, "level", "JSON log should contain `level` field")
				assert.Contains(t, logData, "time", "JSON log should contain `time` field")
				assert.Equal(t, "info", logData["level"], "JSON log should have correct `level`")
				assert.Equal(t, "test-log", logData["message"], "JSON log should contain the logged message")
			}

			if tt.expected == "ecs" {
				assert.Contains(t, logData, "event.dataset", "ECS log should contain `event.dataset`")
				assert.Contains(t, logData, "service.name", "ECS log should contain `service.name`")
				assert.True(t, logData["@timestamp"] != nil || logData["time"] != nil,
					"ECS log should contain `@timestamp` or `time`")
				assert.Equal(t, "info", logData["log.level"], "ECS log should have correct `log.level`")
				assert.Contains(t, logData, "caller", "ECS log should contain `caller` field")
				assert.Equal(t, "test-log", logData["message"], "ECS log should contain the logged message")
			}
		})
	}
}
