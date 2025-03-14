package user

import (
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"github.com/stretchr/testify/assert"
)

func Test_impl_parse_service_account_string(t *testing.T) {
	tests := []struct {
		name string
		user string
		want ServiceAccount
	}{{
		name: "simple",
		user: "system:serviceaccount:foo:bar",
		want: ServiceAccount{Namesapce: "foo", Name: "bar"},
	}, {
		name: "with :",
		user: "system:serviceaccount:foo:bar:baz",
		want: ServiceAccount{Namesapce: "foo", Name: "bar:baz"},
	}, {
		name: "not a service account",
		user: "something-else:123",
		want: ServiceAccount{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Lib()
			env, err := cel.NewEnv(opts)
			assert.NoError(t, err)
			assert.NotNil(t, env)
			ast, issues := env.Compile(fmt.Sprintf(`user.ParseServiceAccount("%s")`, tt.user))
			fmt.Println(issues.String())
			assert.Nil(t, issues)
			assert.NotNil(t, ast)
			prog, err := env.Program(ast)
			assert.NoError(t, err)
			assert.NotNil(t, prog)
			out, _, err := prog.Eval(map[string]any{})
			assert.NoError(t, err)
			sa, err := utils.ConvertToNative[ServiceAccount](out)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, sa)
		})
	}
}
