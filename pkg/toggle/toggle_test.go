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
			got := tr.Enabled()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_StringSliceFlag_Values(t *testing.T) {
	defaults := []string{"10.0.0.0/8", "192.168.0.0/16"}
	tests := []struct {
		name     string
		defaults []string
		parsed   string
		env      map[string]string
		envVar   string
		want     []string
	}{{
		name:     "returns default when nothing set",
		defaults: defaults,
		want:     defaults,
	}, {
		name:     "nil default returns nil",
		defaults: nil,
		want:     nil,
	}, {
		name:   "Parse overrides default",
		parsed: "172.16.0.0/12,127.0.0.0/8",
		want:   []string{"172.16.0.0/12", "127.0.0.0/8"},
	}, {
		name:   "Parse trims whitespace",
		parsed: " 172.16.0.0/12 , 127.0.0.0/8 ",
		want:   []string{"172.16.0.0/12", "127.0.0.0/8"},
	}, {
		name:   "Parse empty string yields empty slice",
		parsed: "",
		want:   nil,
	}, {
		name:   "Parse with only commas and spaces yields empty slice",
		parsed: " , , ",
		want:   []string{},
	}, {
		name:     "env var overrides default",
		defaults: defaults,
		envVar:   "TEST_SLICE_FLAG",
		env:      map[string]string{"TEST_SLICE_FLAG": "1.2.3.4/32"},
		want:     []string{"1.2.3.4/32"},
	}, {
		name:     "parsed value takes precedence over env var",
		defaults: defaults,
		envVar:   "TEST_SLICE_FLAG",
		env:      map[string]string{"TEST_SLICE_FLAG": "1.2.3.4/32"},
		parsed:   "5.6.7.8/32",
		want:     []string{"5.6.7.8/32"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			f := newStringSliceFlag(tt.defaults, tt.envVar)
			if tt.parsed != "" {
				assert.NoError(t, f.Parse(tt.parsed))
			}
			got := f.Values()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_StringSliceFlag_Reset(t *testing.T) {
	f := newStringSliceFlag([]string{"default"}, "")
	assert.NoError(t, f.Parse("parsed"))
	assert.Equal(t, []string{"parsed"}, f.Values())

	f.Reset()
	assert.Equal(t, []string{"default"}, f.Values())
}

func Test_StringSliceFlag_ValuesIsDefensiveCopy(t *testing.T) {
	f := newStringSliceFlag([]string{"original"}, "")
	got := f.Values()
	got[0] = "mutated"
	assert.Equal(t, []string{"original"}, f.Values())
}
