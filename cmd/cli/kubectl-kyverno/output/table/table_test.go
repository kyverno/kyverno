package table

import (
	"reflect"
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
		rows:    []Row{{}, {RowCompact: RowCompact{IsFailure: true}}},
		want:    1,
	}, {
		name:    "two + one",
		RawRows: []Row{{}, {}},
		rows:    []Row{{RowCompact: RowCompact{IsFailure: true}}, {}},
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

func TestTable_Rows(t *testing.T) {
	var nilRows []Row
	var nilCompactRows []RowCompact
	tests := []struct {
		name     string
		RawRows  []Row
		detailed bool
		want     interface{}
	}{{
		name: "nil",
		want: nilCompactRows,
	}, {
		name:     "nil - detailed",
		detailed: true,
		want:     nilRows,
	}, {
		name: "compact",
		RawRows: []Row{{
			RowCompact: RowCompact{
				ID:       1,
				Policy:   "policy1",
				Rule:     "rule1",
				Resource: "resource1",
				Result:   "result1",
				Reason:   "reason1",
			},
			Message: "message1",
		}, {
			RowCompact: RowCompact{
				IsFailure: true,
				ID:        2,
				Policy:    "policy2",
				Rule:      "rule2",
				Resource:  "resource2",
				Result:    "result2",
				Reason:    "reason2",
			},
			Message: "message2",
		}},
		want: []RowCompact{{
			ID:       1,
			Policy:   "policy1",
			Rule:     "rule1",
			Resource: "resource1",
			Result:   "result1",
			Reason:   "reason1",
		}, {
			IsFailure: true,
			ID:        2,
			Policy:    "policy2",
			Rule:      "rule2",
			Resource:  "resource2",
			Result:    "result2",
			Reason:    "reason2",
		}},
	}, {
		name:     "detailed",
		detailed: true,
		RawRows: []Row{{
			RowCompact: RowCompact{
				ID:       1,
				Policy:   "policy1",
				Rule:     "rule1",
				Resource: "resource1",
				Result:   "result1",
				Reason:   "reason1",
			},
			Message: "message1",
		}, {
			RowCompact: RowCompact{
				IsFailure: true,
				ID:        2,
				Policy:    "policy2",
				Rule:      "rule2",
				Resource:  "resource2",
				Result:    "result2",
				Reason:    "reason2",
			},
			Message: "message2",
		}},
		want: []Row{{
			RowCompact: RowCompact{
				ID:       1,
				Policy:   "policy1",
				Rule:     "rule1",
				Resource: "resource1",
				Result:   "result1",
				Reason:   "reason1",
			},
			Message: "message1",
		}, {
			RowCompact: RowCompact{
				IsFailure: true,
				ID:        2,
				Policy:    "policy2",
				Rule:      "rule2",
				Resource:  "resource2",
				Result:    "result2",
				Reason:    "reason2",
			},
			Message: "message2",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Table{
				RawRows: tt.RawRows,
			}
			if got := tr.Rows(tt.detailed); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Table.Rows() = %v, want %v", got, tt.want)
			}
		})
	}
}
