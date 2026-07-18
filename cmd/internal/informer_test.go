package internal

import (
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestCheckCacheSync(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name   string
		status map[reflect.Type]bool
		want   bool
	}{{
		name:   "nil map",
		status: nil,
		want:   true,
	}, {
		name:   "empty map",
		status: map[reflect.Type]bool{},
		want:   true,
	}, {
		name: "all synced",
		status: map[reflect.Type]bool{
			reflect.TypeOf(0):   true,
			reflect.TypeOf(""):  true,
		},
		want: true,
	}, {
		name: "one not synced",
		status: map[reflect.Type]bool{
			reflect.TypeOf(0): false,
		},
		want: false,
	}, {
		name: "mixed synced and not synced",
		status: map[reflect.Type]bool{
			reflect.TypeOf(0):   true,
			reflect.TypeOf(""):  false,
		},
		want: false,
	}, {
		name: "all not synced",
		status: map[reflect.Type]bool{
			reflect.TypeOf(0):   false,
			reflect.TypeOf(""):  false,
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckCacheSync(logger, tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}
