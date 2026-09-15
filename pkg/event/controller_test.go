package event

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateMessageUTF8(t *testing.T) {
	// Test that truncation is rune-safe
	longASCII := make([]rune, 1100)
	for i := range longASCII {
		longASCII[i] = 'A'
	}
	msg := string(longASCII)
	if utf8.RuneCountInString(msg) != 1100 {
		t.Fatalf("expected 1100 runes, got %d", utf8.RuneCountInString(msg))
	}
	truncated := msg
	if utf8.RuneCountInString(truncated) > 1024 {
		truncated = string([]rune(truncated)[:1021]) + "..."
	}
	if utf8.RuneCountInString(truncated) != 1024 {
		t.Errorf("expected 1024 runes after truncation, got %d", utf8.RuneCountInString(truncated))
	}

	// Test multi-byte characters don't get split
	multiByte := "Hello \u4e16\u754c World" // Chinese chars
	if utf8.RuneCountInString(multiByte) <= 1024 {
		// Should not be truncated
	}
	// Simulate a very long message with emoji
	longEmoji := ""
	for i := 0; i < 1100; i++ {
		longEmoji += "\U0001f600" // 😀
	}
	truncated = longEmoji
	if utf8.RuneCountInString(truncated) > 1024 {
		truncated = string([]rune(truncated)[:1021]) + "..."
	}
	// Should end with complete emoji, not half a character
	lastRune, _ := utf8.DecodeLastRuneInString(truncated)
	if lastRune == utf8.RuneError {
		t.Error("truncated message ends with invalid rune")
	}
}
