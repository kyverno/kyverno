package log

import "testing"

func TestConfigure(t *testing.T) {
	if err := Configure(); (err != nil) != false {
		t.Errorf("Configure() error = %v, wantErr %v", err, false)
	}
}
