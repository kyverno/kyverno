package compiler

import (
	"sync"

	"github.com/google/cel-go/cel"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	gceProjectionEnvOnce sync.Once
	gceProjectionEnv     *cel.Env
	errGCEProjectionEnv  error
)

// getGCEProjectionEnv lazily builds, once, the CEL env used to compile GlobalContextEntry
// projection expressions: the base env (stdlib, ext.*, k8s library.*) plus the single
// `object` variable a projection expression is evaluated against. Built once and shared
// because it carries no per-entry state and cel.Env/cel.Program are concurrency-safe;
// rebuilding it per reconcile would register every library in DefaultEnvOptions again on
// every GlobalContextEntry event.
func getGCEProjectionEnv() (*cel.Env, error) {
	gceProjectionEnvOnce.Do(func() {
		opts := append(append([]cel.EnvOption{}, DefaultEnvOptions()...), cel.Variable(ObjectKey, cel.DynType))
		gceProjectionEnv, errGCEProjectionEnv = NewEnv(opts...)
	})
	return gceProjectionEnv, errGCEProjectionEnv
}

// CompileGlobalContextProjection compiles a GlobalContextEntry projection's CEL expression.
// `object` is bound at evaluation time to exactly the value `globalContext.Get(name)`
// returns with no projection applied: the parsed API response for an APICall entry, or the
// list of matching objects for a KubernetesResource entry.
func CompileGlobalContextProjection(path *field.Path, expr string) (cel.Program, field.ErrorList) {
	env, err := getGCEProjectionEnv()
	if err != nil {
		return nil, field.ErrorList{field.InternalError(path, err)}
	}
	ast, err := Compile(env, expr)
	if err != nil {
		return nil, field.ErrorList{field.Invalid(path, expr, err.Error())}
	}
	prog, err := NewProgram(env, ast)
	if err != nil {
		return nil, field.ErrorList{field.Invalid(path, expr, err.Error())}
	}
	return prog, nil
}
