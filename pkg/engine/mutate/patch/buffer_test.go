package patch

import (
	"bytes"
	"reflect"
	"testing"
)

func Test_buffer_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		Buffer  *bytes.Buffer
		b       []byte
		wantErr bool
	}{{
		Buffer: bytes.NewBufferString(""),
		b:      []byte("aaa"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buff := buffer{
				Buffer: tt.Buffer,
			}
			if err := buff.UnmarshalJSON(tt.b); (err != nil) != tt.wantErr {
				t.Errorf("buffer.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_buffer_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		Buffer  *bytes.Buffer
		want    []byte
		wantErr bool
	}{{
		Buffer: bytes.NewBufferString("aaa"),
		want:   []byte("aaa"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buff := buffer{
				Buffer: tt.Buffer,
			}
			got, err := buff.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("buffer.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buffer.MarshalJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}
