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
		name string
		args []string
		want bool
	}{{
		name: "nil",
		args: nil,
		want: false,
	}, {
		name: "empty",
		args: []string{},
		want: false,
	}, {
		name: "not verbose",
		args: []string{"-verbose", "--verbose", "-vv", "--vv"},
		want: false,
	}, {
		name: "verbose",
		args: []string{"-v", "3"},
		want: true,
	}, {
		name: "verbose",
		args: []string{"-v=3"},
		want: true,
	}, {
		name: "verbose",
		args: []string{"--v", "3"},
		want: true,
	}, {
		name: "verbose",
		args: []string{"--v=3"},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isVerbose(tt.args...); got != tt.want {
				t.Errorf("isVerbose() = %v, want %v", got, tt.want)
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
