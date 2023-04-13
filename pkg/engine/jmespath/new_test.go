package jmespath

import (
	"testing"
)

func TestNew(t *testing.T) {
	type args struct {
		query string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			wantErr: true,
		},
		{
			args: args{
				query: "object.something",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newJMESPath(cfg, tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
