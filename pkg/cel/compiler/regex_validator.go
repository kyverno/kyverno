package compiler

import (
	"regexp"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// regexValidatorName is the base apiserver env's own regex-literal validator name
// (k8s.io/apiserver environment.base.go registers cel.ValidateRegexLiterals() at
// compatibility 1.29+, whose Name() is "cel.validator.matches",
// google/cel-go cel/validator.go). cel-go replaces a registered validator with the same
// Name() and Extend clones validators, so registering our own validator under this name in
// every Kyverno env makes it run inside every env.Check, including checks performed by
// vendored compilers.
const regexValidatorName = "cel.validator.matches"

// regexLiteralPatternFunctions lists every function whose literal pattern argument this
// validator gates, with the argument index the guard decorator uses (constGuardArgIndex):
// a member call's checked-AST Args() excludes the target, so the AST-level index is one
// less than the interpreter-level index for member functions.
var regexLiteralPatternFunctions = []string{
	"matches", "regex.extract", "regex.extractAll", "regex.replace", "find", "findAll",
	"random", "random.random",
}

// regexLiteralValidator replaces the apiserver base env's "invalid regex" check with one
// that applies the pattern gate and the unrolled-length walk first, and only then compiles
// the literal for the same "invalid regex" diagnostic the base validator gave.
type regexLiteralValidator struct{}

func (regexLiteralValidator) Name() string { return regexValidatorName }

func (regexLiteralValidator) Validate(_ *cel.Env, _ cel.ValidatorConfig, a *ast.AST, iss *cel.Issues) {
	root := ast.NavigateAST(a)
	for _, function := range regexLiteralPatternFunctions {
		idx, ok := constGuardArgIndex[function]
		if !ok {
			continue
		}
		for _, node := range ast.MatchDescendants(root, ast.FunctionMatcher(function)) {
			call := node.AsCall()
			argIdx := idx
			if call.IsMemberFunction() {
				argIdx--
			}
			args := call.Args()
			if argIdx < 0 || argIdx >= len(args) {
				continue
			}
			litArg := args[argIdx]
			if litArg.Kind() != ast.LiteralKind {
				continue // a non-constant pattern is checked at runtime by the guard
			}
			pattern, ok := litArg.AsLiteral().Value().(string)
			if !ok {
				continue
			}
			if msg := regexLiteralGuard(function, pattern); msg != "" {
				iss.ReportErrorAtID(litArg.ID(), "%s", msg)
				continue
			}
			// Only after the size gate passes do we run the same "invalid regex" check the
			// base validator ran, so a too-large pattern never reaches regexp.Compile.
			if function != "random" && function != "random.random" {
				if _, err := regexp.Compile(pattern); err != nil {
					iss.ReportErrorAtID(litArg.ID(), "invalid %s argument: %v", function, err)
				}
			}
		}
	}
}

// regexLiteralGuard runs the same pattern gate and unrolled-length walk as the runtime
// guard (patternGuard / randomGuard), on a single-element args slice holding just the
// pattern (index 0), and returns a human-readable refusal or "".
func regexLiteralGuard(function, pattern string) string {
	args := []ref.Val{types.String(pattern)}
	var errVal ref.Val
	if function == "random" || function == "random.random" {
		errVal = randomGuard(args)
	} else {
		errVal = patternGuard(0)(args)
	}
	if errVal == nil {
		return ""
	}
	if err, ok := errVal.(*types.Err); ok {
		return err.Error()
	}
	return "regex pattern refused"
}
