package jsonpointer

import (
	"reflect"
	"testing"
)

func TestParsePath(t *testing.T) {
	type args struct {
		rawPath string
	}
	tests := []struct {
		name string
		args args
		want Pointer
	}{
		{
			name: "plain",
			args: args{
				rawPath: "a/b/c",
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "hyphen",
			args: args{
				rawPath: "a/b-b/c",
			},
			want: []string{"a", "b-b", "c"},
		},
		{
			name: "quotes",
			args: args{
				rawPath: `a/"b/b"/c`,
			},
			want: []string{"a", "b/b", "c"},
		},
		{
			name: "escaped_slash",
			args: args{
				rawPath: `a/b\/b/c`,
			},
			want: []string{"a", "b/b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParsePath(tt.args.rawPath); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPointer_Append(t *testing.T) {
	type args struct {
		s []string
	}
	tests := []struct {
		name string
		p    Pointer
		args args
		want Pointer
	}{
		{
			p: []string{"a", "b"},
			args: args{
				s: []string{"c", "d"},
			},
			want: []string{"a", "b", "c", "d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Append(tt.args.s...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Append() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPointer_AppendPath(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		p    Pointer
		args args
		want Pointer
	}{
		{
			name: "",
			p:    []string{"a", "b", "c"},
			args: args{
				s: `d/e\/e/f`,
			},
			want: []string{"a", "b", "c", "d", "e/e", "f"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.AppendPath(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AppendPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPointer_JMESPath(t *testing.T) {
	tests := []struct {
		name string
		p    Pointer
		want string
	}{
		{
			p:    []string{"a", "b", "c", "3", "e/e", "f"},
			want: `a.b.c[3]."e/e".f`,
		},
		{
			p:    []string{"a", "b", "c", "3", "e/e", "f"},
			want: `a.b.c[3]."e/e".f`,
		},
		{
			name: "hangul",
			p:    []string{"a", "바나나", "c", "3", "e/e", "f"},
			want: `a."바나나".c[3]."e/e".f`,
		},
		{
			name: "tab",
			p:    []string{"a", "a\tb", "c"},
			want: `a."a\tb".c`,
		},
		{
			name: "bell",
			p:    []string{"a", "a\aa", "c", "3", "e/e", "f"},
			want: `a."a\u0007a".c[3]."e/e".f`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.JMESPath(); got != tt.want {
				t.Errorf("JMESPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPointer_String(t *testing.T) {
	tests := []struct {
		name string
		p    Pointer
		want string
	}{
		{
			p:    []string{"a", "b", "c"},
			want: "a/b/c",
		},
		{
			p:    []string{"a", "b/b", "c~c"},
			want: `a/b~1b/c~0c`,
		},
		{
			p:    []string{"a", `b\b`, `c"c`},
			want: `a/b\\b/c\"c`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPointer_Prepend(t *testing.T) {
	type args struct {
		s []string
	}
	tests := []struct {
		name string
		p    Pointer
		args args
		want Pointer
	}{
		{
			p: []string{"c", "d", "e"},
			args: args{
				s: []string{"a", "b"},
			},
			want: []string{"a", "b", "c", "d", "e"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Prepend(tt.args.s...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Prepend() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want Pointer
	}{
		{
			args: args{
				s: "a/b~1c/~0d",
			},
			want: []string{"a", "b/c", "~d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
