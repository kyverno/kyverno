package json

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPatchOperation(t *testing.T) {
	type args struct {
		path  string
		op    string
		value interface{}
	}
	tests := []struct {
		name string
		args args
		want PatchOperation
	}{{
		name: "test",
		args: args{"path", "op", 123},
		want: PatchOperation{
			Path:  "path",
			Op:    "op",
			Value: 123,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NewPatchOperation(tt.args.path, tt.args.op, tt.args.value))
		})
	}
}

func TestPatchOperation_Marshal(t *testing.T) {
	type fields struct {
		Path  string
		Op    string
		Value interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{{
		name: "test",
		fields: fields{
			Path:  "path",
			Op:    "op",
			Value: 123,
		},
		want:    []byte(`{"path":"path","op":"op","value":123}`),
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PatchOperation{
				Path:  tt.fields.Path,
				Op:    tt.fields.Op,
				Value: tt.fields.Value,
			}
			got, err := p.Marshal()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPatchOperation_ToPatchBytes(t *testing.T) {
	type fields struct {
		Path  string
		Op    string
		Value interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{{
		name: "test",
		fields: fields{
			Path:  "path",
			Op:    "op",
			Value: 123,
		},
		want:    []byte(`[{"path":"path","op":"op","value":123}]`),
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PatchOperation{
				Path:  tt.fields.Path,
				Op:    tt.fields.Op,
				Value: tt.fields.Value,
			}
			got, err := p.ToPatchBytes()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestMarshalPatchOperation(t *testing.T) {
	type args struct {
		path  string
		op    string
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{{
		name: "test",
		args: args{
			path:  "path",
			op:    "op",
			value: 123,
		},
		want:    []byte(`{"path":"path","op":"op","value":123}`),
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MarshalPatchOperation(tt.args.path, tt.args.op, tt.args.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCheckPatch(t *testing.T) {
	type args struct {
		patch []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{{
		name:    "test",
		args:    args{[]byte(`{"path":"path","op":"add","value":123}`)},
		wantErr: false,
	}, {
		name:    "error",
		args:    args{[]byte(`"foo":"bar"`)},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPatch(tt.args.patch)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalPatchOperation(t *testing.T) {
	type args struct {
		patch []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *PatchOperation
		wantErr bool
	}{{
		name: "test",
		args: args{[]byte(`{"path":"path","op":"op","value":123}`)},
		want: &PatchOperation{
			Path:  "path",
			Op:    "op",
			Value: float64(123),
		},
		wantErr: false,
	}, {
		name:    "error",
		args:    args{[]byte(`"foo":"bar"`)},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalPatchOperation(tt.args.patch)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
