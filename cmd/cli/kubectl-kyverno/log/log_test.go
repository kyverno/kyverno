package log

import (
	"testing"
)

func TestConfigure(t *testing.T) {
	if err := Configure(); (err != nil) != false {
		t.Errorf("Configure() error = %v, wantErr %v", err, false)
	}
}

func Test_isVerbose(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		want  bool
		level int
	}{{
		name:  "nil",
		args:  nil,
		want:  false,
		level: 0,
	}, {
		name:  "empty",
		args:  []string{},
		want:  false,
		level: 0,
	}, {
		name:  "not verbose",
		args:  []string{"-verbose", "--verbose", "-vv", "--vv"},
		want:  false,
		level: 0,
	}, {
		name:  "verbose",
		args:  []string{"-v", "3"},
		want:  true,
		level: 3,
	}, {
		name:  "verbose",
		args:  []string{"-v"},
		want:  true,
		level: defaultLogLevel,
	}, {
		name:  "verbose",
		args:  []string{"-v=3"},
		want:  true,
		level: 3,
	}, {
		name:  "verbose",
		args:  []string{"--v", "3"},
		want:  true,
		level: 3,
	}, {
		name:  "verbose",
		args:  []string{"--v=3"},
		want:  true,
		level: 3,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, level, _ := isVerbose(tt.args...)
			if got != tt.want {
				t.Errorf("isVerbose() = %v, want %v", got, tt.want)
			}
			if level != tt.level {
				t.Errorf("isVerbose() level = %v, want %v", level, tt.level)
			}
		})
	}
}

func Test_configure(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{{
		name:    "nil",
		args:    nil,
		wantErr: false,
	}, {
		name:    "empty",
		args:    []string{},
		wantErr: false,
	}, {
		name:    "not verbose",
		args:    []string{"-verbose", "--verbose", "-vv", "--vv"},
		wantErr: false,
	}, {
		name:    "verbose",
		args:    []string{"-v", "3"},
		wantErr: false,
	}, {
		name:    "verbose",
		args:    []string{"-v=3"},
		wantErr: false,
	}, {
		name:    "verbose",
		args:    []string{"--v", "3"},
		wantErr: false,
	}, {
		name:    "verbose",
		args:    []string{"--v=3"},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := configure(tt.args...); (err != nil) != tt.wantErr {
				t.Errorf("configure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
