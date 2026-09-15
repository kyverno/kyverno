package event

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestTruncateMessageToByteLimit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxBytes int
		want     string
	}{
		{
			name:     "short ASCII message",
			input:    "hello",
			maxBytes: 1024,
			want:     "hello",
		},
		{
			name:     "ASCII exactly at limit",
			input:    string(make([]byte, 1024)),
			maxBytes: 1024,
			want:     string(make([]byte, 1024)),
		},
		{
			name:     "ASCII exceeds limit",
			input:    string(make([]byte, 1100)),
			maxBytes: 1024,
			want:     string(make([]byte, 1021)) + "...",
		},
		{
			name:     "multi-byte characters exact boundary",
			input:    "Hello World",
			maxBytes: 11,
			want:     "Hello World",
		},
		{
			name:     "multi-byte characters truncated at rune boundary",
			input:    "Hello 世界",
			maxBytes: 8,
			want:     "Hello ...",
		},
		{
			name:     "emoji truncated at valid boundary",
			input:    "😀😀😀",
			maxBytes: 8,
			want:     "😀....",
		},
		{
			name:     "CJK characters truncated correctly",
			input:    "你好世界",
			maxBytes: 7,
			want:     "你...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateMessageToByteLimit(tt.input, tt.maxBytes)
			assert.Equal(t, tt.want, got)
			// Verify result is valid UTF-8
			assert.True(t, utf8.ValidString(got), "result should be valid UTF-8")
			// Verify length constraint
			assert.LessOrEqual(t, len(got), tt.maxBytes, "result should not exceed maxBytes")
		})
	}
}

func TestTruncateMessageWithLongEmoji(t *testing.T) {
	// Create a long message with emoji (4 bytes each)
	longEmoji := ""
	for i := 0; i < 300; i++ {
		longEmoji += "😀" // 4 bytes per emoji
	}

	result := truncateMessageToByteLimit(longEmoji, 1024)
	assert.LessOrEqual(t, len(result), 1024)
	assert.True(t, utf8.ValidString(result))
	assert.NotEmpty(t, result)
}

func TestTruncateMessageWithMixedContent(t *testing.T) {
	// Mixed ASCII and multi-byte characters
	mixed := "Hello 世界 Hello 世界"
	result := truncateMessageToByteLimit(mixed, 15)
	assert.True(t, utf8.ValidString(result))
	assert.LessOrEqual(t, len(result), 15)
}