package table

import (
	"os"
	"testing"
)

func TestNewTablePrinter(t *testing.T) {
	if got := NewTablePrinter(os.Stdout); got == nil {
		t.Errorf("NewTablePrinter() return nill")
	}
}

func Test_rowsLength(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   bool
	}{{
		name:   "0",
		length: 0,
		want:   false,
	}, {
		name:   "10",
		length: 10,
		want:   false,
	}, {
		name:   "11",
		length: 11,
		want:   true,
	}, {
		name:   "20",
		length: 20,
		want:   true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rowsLength(tt.length); got != tt.want {
				t.Errorf("rowsLength() = %v, want %v", got, tt.want)
			}
		})
	}
}
