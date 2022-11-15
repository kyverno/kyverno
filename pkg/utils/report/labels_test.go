package report

import "testing"

func TestCompareResourceVersion(t *testing.T) {
	type args struct {
		current string
		new     string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "all empty",
			args: args{
				current: "",
				new:     "",
			},
			want: "",
		},
		{
			name: "current empty",
			args: args{
				current: "",
				new:     "1",
			},
			want: "1",
		},
		{
			name: "new empty",
			args: args{
				current: "1",
				new:     "",
			},
			want: "1",
		},
		{
			name: "new greater",
			args: args{
				current: "1",
				new:     "2",
			},
			want: "2",
		},
		{
			name: "current greater",
			args: args{
				current: "9",
				new:     "5",
			},
			want: "9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompareResourceVersion(tt.args.current, tt.args.new); got != tt.want {
				t.Errorf("CompareResourceVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
