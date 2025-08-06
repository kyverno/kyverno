package user

import (
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
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
		want: ServiceAccount{Namespace: "foo", Name: "bar"},
	}, {
		name: "with :",
		user: "system:serviceaccount:foo:bar:baz",
		want: ServiceAccount{Namespace: "foo", Name: "bar:baz"},
	}, {
		name: "not a service account",
		user: "something-else:123",
		want: ServiceAccount{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, err := compiler.NewBaseEnv()
			assert.NoError(t, err)
			assert.NotNil(t, base)
			options := []cel.EnvOption{
				Lib(),
			}
			env, err := base.Extend(options...)
			assert.NoError(t, err)
			assert.NotNil(t, env)
			ast, issues := env.Compile(fmt.Sprintf(`parseServiceAccount("%s")`, tt.user))
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

func Test_impl_parse_service_account_string_error(t *testing.T) {
	tests := []struct {
		name string
		user ref.Val
		want ref.Val
	}{{
		name: "bad arg",
		user: types.Bool(false),
		want: types.NewErr("type conversion error from bool to 'string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.parse_service_account_string(tt.user)
			assert.Equal(t, tt.want, got)
		})
	}
}
