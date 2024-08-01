package validation

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/jmespath-community/go-jmespath/pkg/binding"
	"github.com/jmespath-community/go-jmespath/pkg/functions"
	jpfunctions "github.com/jmespath-community/go-jmespath/pkg/functions"
	"github.com/jmespath-community/go-jmespath/pkg/interpreter"
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno-json/pkg/engine/assert"
	"github.com/kyverno/kyverno-json/pkg/engine/template"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func getArgAt(arguments []any, index int) (any, error) {
	if index >= len(arguments) {
		return nil, fmt.Errorf("index out of range (%d / %d)", index, len(arguments))
	}
	return arguments[index], nil
}

func getArg[T any](arguments []any, index int, out *T) error {
	arg, err := getArgAt(arguments, index)
	if err != nil {
		return err
	}
	if value, ok := arg.(T); !ok {
		return errors.New("invalid type")
	} else {
		*out = value
		return nil
	}
}

func jpContextQuery(arguments []any) (any, error) {
	var ctx enginectx.EvalInterface
	var query string
	if err := getArg(arguments, 0, &ctx); err != nil {
		return nil, err
	}
	if err := getArg(arguments, 1, &query); err != nil {
		return nil, err
	}
	return ctx.Query(query)
}

func jpContextHasChanged(arguments []any) (any, error) {
	var ctx enginectx.EvalInterface
	var query string
	if err := getArg(arguments, 0, &ctx); err != nil {
		return nil, err
	}
	if err := getArg(arguments, 1, &query); err != nil {
		return nil, err
	}
	return ctx.HasChanged(query)
}

var caller = sync.OnceValue(func() interpreter.FunctionCaller {
	var funcs []jpfunctions.FunctionEntry
	funcs = append(funcs, template.GetFunctions(context.Background())...)
	funcs = append(funcs, jpfunctions.FunctionEntry{
		Name: "context_query",
		Arguments: []functions.ArgSpec{
			{Types: []functions.JpType{functions.JpAny}},
			{Types: []functions.JpType{functions.JpString}},
		},
		Handler: jpContextQuery,
	})
	funcs = append(funcs, jpfunctions.FunctionEntry{
		Name: "context_has_changed",
		Arguments: []functions.ArgSpec{
			{Types: []functions.JpType{functions.JpAny}},
			{Types: []functions.JpType{functions.JpString}},
		},
		Handler: jpContextHasChanged,
	})
	return interpreter.NewFunctionCaller(funcs...)
})

type validateAssertHandler struct{}

func NewValidateAssertHandler() (handlers.Handler, error) {
	return validateAssertHandler{}, nil
}

func (h validateAssertHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	contextLoader engineapi.EngineContextLoader,
	exceptions []*kyvernov2.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// check if there are policy exceptions that match the incoming resource
	matchedExceptions := engineutils.MatchesException(exceptions, policyContext, logger)
	if len(matchedExceptions) > 0 {
		var keys []string
		for i, exception := range matchedExceptions {
			key, err := cache.MetaNamespaceKeyFunc(&matchedExceptions[i])
			if err != nil {
				logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
				return resource, handlers.WithError(rule, engineapi.Validation, "failed to compute exception key", err)
			}
			keys = append(keys, key)
		}
		logger.V(3).Info("policy rule is skipped due to policy exceptions", "exceptions", keys)
		return resource, handlers.WithResponses(
			engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule is skipped due to policy exceptions"+strings.Join(keys, ", ")).WithExceptions(matchedExceptions),
		)
	}
	// load context
	jsonContext := policyContext.JSONContext()
	jsonContext.Checkpoint()
	defer jsonContext.Restore()
	if err := contextLoader(ctx, rule.Context, jsonContext); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			logger.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			logger.Error(err, "failed to load context")
		}
		return resource, handlers.WithResponses(
			engineapi.RuleError(rule.Name, engineapi.Validation, "failed to load context", err),
		)
	}
	// execute assertion
	assertion := rule.Validation.Assert
	gvk, subResource := policyContext.ResourceKind()
	bindings := binding.NewBindings()
	bindings = bindings.Register("$kyverno", binding.NewBinding(map[string]any{
		"context":            jsonContext,
		"oldObject":          policyContext.OldResource().Object,
		"admissionInfo":      policyContext.AdmissionInfo(),
		"operation":          policyContext.Operation(),
		"namespaceLabels":    policyContext.NamespaceLabels(),
		"admissionOperation": policyContext.AdmissionOperation(),
		"requestResource":    policyContext.RequestResource(),
		"resourceKind": map[string]any{
			"group":       gvk.Group,
			"version":     gvk.Version,
			"kind":        gvk.Kind,
			"subResource": subResource,
		},
	}))
	errs, err := assert.Assert(
		ctx,
		nil,
		assert.Parse(ctx, assertion.Value),
		resource.UnstructuredContent(),
		bindings,
		template.WithFunctionCaller(caller()),
	)
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "failed to apply assertion", err)
	}
	// compose a response
	if len(errs) != 0 {
		var responses []*engineapi.RuleResponse
		for _, err := range errs {
			responses = append(responses, engineapi.RuleFail(rule.Name, engineapi.Validation, err.Error()))
		}
		return resource, handlers.WithResponses(responses...)
	}
	msg := fmt.Sprintf("Validation rule '%s' passed.", rule.Name)
	return resource, handlers.WithResponses(
		engineapi.RulePass(rule.Name, engineapi.Validation, msg),
	)
}
