package anchor

import (
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	type args struct {
		modifier AnchorType
		key      string
	}
	tests := []struct {
		name string
		args args
		want Anchor
	}{{
		args: args{Condition, ""},
		want: nil,
	}, {
		args: args{Global, ""},
		want: nil,
	}, {
		args: args{Negation, ""},
		want: nil,
	}, {
		args: args{AddIfNotPresent, ""},
		want: nil,
	}, {
		args: args{Equality, ""},
		want: nil,
	}, {
		args: args{Existence, ""},
		want: nil,
	}, {
		args: args{Condition, "test"},
		want: anchor{Condition, "test"},
	}, {
		args: args{Global, "test"},
		want: anchor{Global, "test"},
	}, {
		args: args{Negation, "test"},
		want: anchor{Negation, "test"},
	}, {
		args: args{AddIfNotPresent, "test"},
		want: anchor{AddIfNotPresent, "test"},
	}, {
		args: args{Equality, "test"},
		want: anchor{Equality, "test"},
	}, {
		args: args{Existence, "test"},
		want: anchor{Existence, "test"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.modifier, tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	type args struct {
		modifier AnchorType
		key      string
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		args: args{Condition, ""},
		want: "",
	}, {
		args: args{Global, ""},
		want: "",
	}, {
		args: args{Negation, ""},
		want: "",
	}, {
		args: args{AddIfNotPresent, ""},
		want: "",
	}, {
		args: args{Equality, ""},
		want: "",
	}, {
		args: args{Existence, ""},
		want: "",
	}, {
		args: args{Condition, "test"},
		want: "(test)",
	}, {
		args: args{Global, "test"},
		want: "<(test)",
	}, {
		args: args{Negation, "test"},
		want: "X(test)",
	}, {
		args: args{AddIfNotPresent, "test"},
		want: "+(test)",
	}, {
		args: args{Equality, "test"},
		want: "=(test)",
	}, {
		args: args{Existence, "test"},
		want: "^(test)",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := String(tt.args.modifier, tt.args.key); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOneOf(t *testing.T) {
	type args struct {
		a     Anchor
		types []AnchorType
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{},
		want: false,
	}, {
		args: args{nil, []AnchorType{Condition, Negation}},
		want: false,
	}, {
		args: args{New(Condition, "test"), nil},
		want: false,
	}, {
		args: args{New(Condition, "test"), []AnchorType{}},
		want: false,
	}, {
		args: args{New(Condition, "test"), []AnchorType{Condition}},
		want: true,
	}, {
		args: args{New(Condition, "test"), []AnchorType{Condition, Negation}},
		want: true,
	}, {
		args: args{New(Condition, "test"), []AnchorType{Negation, Global}},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOneOf(tt.args.a, tt.args.types...); got != tt.want {
				t.Errorf("IsOneOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want Anchor
	}{
		{
			args: args{"(something)"},
			want: anchor{Condition, "something"},
		}, {
			args: args{"()"},
			want: nil,
		}, {
			args: args{"something"},
			want: nil,
		}, {
			args: args{"(something"},
			want: nil,
		}, {
			args: args{"something)"},
			want: nil,
		}, {
			args: args{"so)m(et(hin)g"},
			want: nil,
		}, {
			args: args{""},
			want: nil,
		}, {
			args: args{"^(abc)"},
			want: anchor{Existence, "abc"},
		}, {
			args: args{"^(abc"},
			want: nil,
		}, {
			args: args{"^abc"},
			want: nil,
		}, {
			args: args{"^()"},
			want: nil,
		}, {
			args: args{"(abc)"},
			want: anchor{Condition, "abc"},
		}, {
			args: args{"=(abc)"},
			want: anchor{Equality, "abc"},
		}, {
			args: args{"=(abc"},
			want: nil,
		}, {
			args: args{"=abc"},
			want: nil,
		}, {
			args: args{"+(abc)"},
			want: anchor{AddIfNotPresent, "abc"},
		}, {
			args: args{"+(abc"},
			want: nil,
		}, {
			args: args{"+abc"},
			want: nil,
		}, {
			args: args{"X(abc)"},
			want: anchor{Negation, "abc"},
		}, {
			args: args{"X(abc"},
			want: nil,
		}, {
			args: args{"Xabc"},
			want: nil,
		}, {
			args: args{"<(abc)"},
			want: anchor{Global, "abc"},
		}, {
			args: args{"<(abc"},
			want: nil,
		}, {
			args: args{"<abc"},
			want: nil,
		}, {
			args: args{"(abc)"},
			want: anchor{Condition, "abc"},
		}, {
			args: args{"(abc"},
			want: nil,
		}, {
			args: args{"abc"},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.args.str); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_anchor_Type(t *testing.T) {
	type fields struct {
		modifier AnchorType
		key      string
	}
	tests := []struct {
		name   string
		fields fields
		want   AnchorType
	}{{
		fields: fields{Condition, "abc"},
		want:   Condition,
	}, {
		fields: fields{Global, "abc"},
		want:   Global,
	}, {
		fields: fields{Negation, "abc"},
		want:   Negation,
	}, {
		fields: fields{AddIfNotPresent, "abc"},
		want:   AddIfNotPresent,
	}, {
		fields: fields{Equality, "abc"},
		want:   Equality,
	}, {
		fields: fields{Existence, "abc"},
		want:   Existence,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := anchor{
				modifier: tt.fields.modifier,
				key:      tt.fields.key,
			}
			if got := a.Type(); got != tt.want {
				t.Errorf("anchor.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_anchor_Key(t *testing.T) {
	type fields struct {
		modifier AnchorType
		key      string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{{
		fields: fields{Condition, "abc"},
		want:   "abc",
	}, {
		fields: fields{Global, "abc"},
		want:   "abc",
	}, {
		fields: fields{Negation, "abc"},
		want:   "abc",
	}, {
		fields: fields{AddIfNotPresent, "abc"},
		want:   "abc",
	}, {
		fields: fields{Equality, "abc"},
		want:   "abc",
	}, {
		fields: fields{Existence, "abc"},
		want:   "abc",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := anchor{
				modifier: tt.fields.modifier,
				key:      tt.fields.key,
			}
			if got := a.Key(); got != tt.want {
				t.Errorf("anchor.Key() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_anchor_String(t *testing.T) {
	type fields struct {
		modifier AnchorType
		key      string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{{
		fields: fields{Condition, "abc"},
		want:   "(abc)",
	}, {
		fields: fields{Global, "abc"},
		want:   "<(abc)",
	}, {
		fields: fields{Negation, "abc"},
		want:   "X(abc)",
	}, {
		fields: fields{AddIfNotPresent, "abc"},
		want:   "+(abc)",
	}, {
		fields: fields{Equality, "abc"},
		want:   "=(abc)",
	}, {
		fields: fields{Existence, "abc"},
		want:   "^(abc)",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := anchor{
				modifier: tt.fields.modifier,
				key:      tt.fields.key,
			}
			if got := a.String(); got != tt.want {
				t.Errorf("anchor.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCondition(t *testing.T) {
	type args struct {
		a Anchor
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{nil},
		want: false,
	}, {
		args: args{New(Condition, "abc")},
		want: true,
	}, {
		args: args{New(Global, "abc")},
		want: false,
	}, {
		args: args{New(Negation, "abc")},
		want: false,
	}, {
		args: args{New(AddIfNotPresent, "abc")},
		want: false,
	}, {
		args: args{New(Equality, "abc")},
		want: false,
	}, {
		args: args{New(Existence, "abc")},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCondition(tt.args.a); got != tt.want {
				t.Errorf("IsCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsGlobal(t *testing.T) {
	type args struct {
		a Anchor
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{nil},
		want: false,
	}, {
		args: args{New(Condition, "abc")},
		want: false,
	}, {
		args: args{New(Global, "abc")},
		want: true,
	}, {
		args: args{New(Negation, "abc")},
		want: false,
	}, {
		args: args{New(AddIfNotPresent, "abc")},
		want: false,
	}, {
		args: args{New(Equality, "abc")},
		want: false,
	}, {
		args: args{New(Existence, "abc")},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGlobal(tt.args.a); got != tt.want {
				t.Errorf("IsGlobal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNegation(t *testing.T) {
	type args struct {
		a Anchor
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{nil},
		want: false,
	}, {
		args: args{New(Condition, "abc")},
		want: false,
	}, {
		args: args{New(Global, "abc")},
		want: false,
	}, {
		args: args{New(Negation, "abc")},
		want: true,
	}, {
		args: args{New(AddIfNotPresent, "abc")},
		want: false,
	}, {
		args: args{New(Equality, "abc")},
		want: false,
	}, {
		args: args{New(Existence, "abc")},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNegation(tt.args.a); got != tt.want {
				t.Errorf("IsNegation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAddIfNotPresent(t *testing.T) {
	type args struct {
		a Anchor
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{nil},
		want: false,
	}, {
		args: args{New(Condition, "abc")},
		want: false,
	}, {
		args: args{New(Global, "abc")},
		want: false,
	}, {
		args: args{New(Negation, "abc")},
		want: false,
	}, {
		args: args{New(AddIfNotPresent, "abc")},
		want: true,
	}, {
		args: args{New(Equality, "abc")},
		want: false,
	}, {
		args: args{New(Existence, "abc")},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAddIfNotPresent(tt.args.a); got != tt.want {
				t.Errorf("IsAddIfNotPresent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsEquality(t *testing.T) {
	type args struct {
		a Anchor
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{nil},
		want: false,
	}, {
		args: args{New(Condition, "abc")},
		want: false,
	}, {
		args: args{New(Global, "abc")},
		want: false,
	}, {
		args: args{New(Negation, "abc")},
		want: false,
	}, {
		args: args{New(AddIfNotPresent, "abc")},
		want: false,
	}, {
		args: args{New(Equality, "abc")},
		want: true,
	}, {
		args: args{New(Existence, "abc")},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEquality(tt.args.a); got != tt.want {
				t.Errorf("IsEquality() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsExistence(t *testing.T) {
	type args struct {
		a Anchor
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{nil},
		want: false,
	}, {
		args: args{New(Condition, "abc")},
		want: false,
	}, {
		args: args{New(Global, "abc")},
		want: false,
	}, {
		args: args{New(Negation, "abc")},
		want: false,
	}, {
		args: args{New(AddIfNotPresent, "abc")},
		want: false,
	}, {
		args: args{New(Equality, "abc")},
		want: false,
	}, {
		args: args{New(Existence, "abc")},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsExistence(tt.args.a); got != tt.want {
				t.Errorf("IsExistence() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsCondition(t *testing.T) {
	type args struct {
		a Anchor
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{nil},
		want: false,
	}, {
		args: args{New(Condition, "abc")},
		want: true,
	}, {
		args: args{New(Global, "abc")},
		want: true,
	}, {
		args: args{New(Negation, "abc")},
		want: false,
	}, {
		args: args{New(AddIfNotPresent, "abc")},
		want: false,
	}, {
		args: args{New(Equality, "abc")},
		want: false,
	}, {
		args: args{New(Existence, "abc")},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsCondition(tt.args.a); got != tt.want {
				t.Errorf("ContainsCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}
