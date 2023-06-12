package toggle

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_newToggle(t *testing.T) {
	type args struct {
		defaultValue bool
		envVar       string
	}
	tests := []struct {
		name string
		args args
		want *toggle
	}{{
		name: "nothing set",
		want: &toggle{},
	}, {
		name: "default value",
		args: args{
			defaultValue: true,
		},
		want: &toggle{
			defaultValue: true,
		},
	}, {
		name: "env var",
		args: args{
			envVar: "test",
		},
		want: &toggle{
			envVar: "test",
		},
	}, {
		name: "all",
		args: args{
			defaultValue: true,
			envVar:       "test",
		},
		want: &toggle{
			defaultValue: true,
			envVar:       "test",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newToggle(tt.args.defaultValue, tt.args.envVar)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_toggle_Parse(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{{
		name:    "empty",
		wantErr: false,
	}, {
		name:    "true",
		args:    args{"true"},
		wantErr: false,
	}, {
		name:    "false",
		args:    args{"false"},
		wantErr: false,
	}, {
		name:    "not a bool",
		args:    args{"test"},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newToggle(false, "")
			err := tr.Parse(tt.args.in)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_toggle_Enabled(t *testing.T) {
	type fields struct {
		defaultValue bool
		envVar       string
	}
	tests := []struct {
		name   string
		fields fields
		value  string
		env    map[string]string
		want   bool
	}{{
		name: "empty",
		want: false,
	}, {
		name: "default true",
		fields: fields{
			defaultValue: true,
		},
		want: true,
	}, {
		name: "default false",
		fields: fields{
			defaultValue: false,
		},
		want: false,
	}, {
		name: "parse true",
		fields: fields{
			defaultValue: false,
		},
		value: "true",
		want:  true,
	}, {
		name: "parse false",
		fields: fields{
			defaultValue: true,
		},
		value: "false",
		want:  false,
	}, {
		name: "env true",
		fields: fields{
			defaultValue: false,
			envVar:       "TOGGLE_FLAG",
		},
		env: map[string]string{
			"TOGGLE_FLAG": "true",
		},
		want: true,
	}, {
		name: "env false",
		fields: fields{
			defaultValue: true,
			envVar:       "TOGGLE_FLAG",
		},
		env: map[string]string{
			"TOGGLE_FLAG": "false",
		},
		want: false,
	}, {
		name: "value takes precedence on env var",
		fields: fields{
			defaultValue: false,
			envVar:       "TOGGLE_FLAG",
		},
		value: "true",
		env: map[string]string{
			"TOGGLE_FLAG": "false",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			tr := newToggle(tt.fields.defaultValue, tt.fields.envVar)
			tr.Parse(tt.value)
			got := tr.enabled()
			assert.Equal(t, tt.want, got)
		})
	}
}
