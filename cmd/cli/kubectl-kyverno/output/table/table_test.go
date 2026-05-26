package table

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTable_Add(t *testing.T) {
	tests := []struct {
		name    string
		RawRows []Row
		rows    []Row
		want    int
	}{{
		name:    "nil",
		RawRows: nil,
		rows:    nil,
		want:    0,
	}, {
		name:    "empty",
		RawRows: nil,
		rows:    []Row{},
		want:    0,
	}, {
		name:    "two",
		RawRows: nil,
		rows:    []Row{{}, {}},
		want:    2,
	}, {
		name:    "two + two",
		RawRows: []Row{{}, {}},
		rows:    []Row{{}, {}},
		want:    4,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Table{
				RawRows: tt.RawRows,
			}
			tr.Add(tt.rows...)
			assert.Equal(t, len(tr.RawRows), tt.want)
		})
	}
}

func TestTable_AddFailed(t *testing.T) {
	tests := []struct {
		name    string
		RawRows []Row
		rows    []Row
		want    int
	}{{
		name:    "nil",
		RawRows: nil,
		rows:    nil,
		want:    0,
	}, {
		name:    "empty",
		RawRows: nil,
		rows:    []Row{},
		want:    0,
	}, {
		name:    "one",
		RawRows: nil,
		rows:    []Row{{}, {IsFailure: true}},
		want:    1,
	}, {
		name:    "two + one",
		RawRows: []Row{{}, {}},
		rows:    []Row{{IsFailure: true}, {}},
		want:    3,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Table{
				RawRows: tt.RawRows,
			}
			tr.AddFailed(tt.rows...)
			assert.Equal(t, len(tr.RawRows), tt.want)
		})
	}
}
