package time

import (
	gotime "time"
	"testing"

	"github.com/google/cel-go/common/types"
)

func TestTimeNow(t *testing.T) {
	result := time_now()
	if _, ok := result.(types.String); !ok {
		t.Fatalf("expected types.String, got %T", result)
	}
	// result should be a valid RFC3339 string
	s := string(result.(types.String))
	if _, err := gotime.Parse(gotime.RFC3339, s); err != nil {
		t.Fatalf("time_now returned invalid RFC3339: %v", err)
	}
}

func TestTimeNowUtc(t *testing.T) {
	result := time_now_utc()
	if _, ok := result.(types.String); !ok {
		t.Fatalf("expected types.String, got %T", result)
	}
	s := string(result.(types.String))
	parsed, err := gotime.Parse(gotime.RFC3339, s)
	if err != nil {
		t.Fatalf("time_now_utc returned invalid RFC3339: %v", err)
	}
	if parsed.Location() != gotime.UTC {
		t.Fatalf("expected UTC, got %v", parsed.Location())
	}
}

func TestTimeParse(t *testing.T) {
	result := time_parse(
		types.String(gotime.RFC3339),
		types.String("2024-01-15T10:00:00Z"),
	)
	if _, ok := result.(types.String); !ok {
		t.Fatalf("expected types.String, got %T", result)
	}
}

func TestTimeParseInvalid(t *testing.T) {
	result := time_parse(
		types.String(gotime.RFC3339),
		types.String("not-a-timestamp"),
	)
	if _, ok := result.(*types.Err); !ok {
		t.Fatalf("expected error for invalid timestamp, got %T", result)
	}
}

func TestTimeAdd(t *testing.T) {
	result := time_add(
		types.String("2024-01-15T10:00:00Z"),
		types.String("24h"),
	)
	if _, ok := result.(types.String); !ok {
		t.Fatalf("expected types.String, got %T", result)
	}
	expected := "2024-01-16T10:00:00Z"
	if string(result.(types.String)) != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestTimeDiff(t *testing.T) {
	result := time_diff(
		types.String("2024-01-15T10:00:00Z"),
		types.String("2024-01-16T10:00:00Z"),
	)
	if _, ok := result.(types.String); !ok {
		t.Fatalf("expected types.String, got %T", result)
	}
	if string(result.(types.String)) != "24h0m0s" {
		t.Fatalf("expected 24h0m0s, got %s", result)
	}
}

func TestTimeBefore(t *testing.T) {
	result := time_before(
		types.String("2024-01-15T10:00:00Z"),
		types.String("2024-01-16T10:00:00Z"),
	)
	if result != types.True {
		t.Fatalf("expected true, got %v", result)
	}

	result = time_before(
		types.String("2024-01-16T10:00:00Z"),
		types.String("2024-01-15T10:00:00Z"),
	)
	if result != types.False {
		t.Fatalf("expected false, got %v", result)
	}
}

func TestTimeAfter(t *testing.T) {
	result := time_after(
		types.String("2024-01-16T10:00:00Z"),
		types.String("2024-01-15T10:00:00Z"),
	)
	if result != types.True {
		t.Fatalf("expected true, got %v", result)
	}

	result = time_after(
		types.String("2024-01-15T10:00:00Z"),
		types.String("2024-01-16T10:00:00Z"),
	)
	if result != types.False {
		t.Fatalf("expected false, got %v", result)
	}
}