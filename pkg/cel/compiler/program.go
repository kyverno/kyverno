package compiler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	"k8s.io/klog/v2"
)

// NewProgram is a thin, drop-in wrapper around env.Program: the per-program cost limit,
// the interrupt frequency, cost tracking, and the guard decorator all reach every program
// built from a Kyverno env automatically, through CostLib's ProgramOptions (costlib.go) --
// this wrapper's only job is to translate cel-go's own cost-limit cancellation into a
// sentinel IsResourceError recognizes, and to surface an unexpected internal error.
func NewProgram(env *cel.Env, ast *cel.Ast, opts ...cel.ProgramOption) (cel.Program, error) {
	prg, err := env.Program(ast, opts...)
	if err != nil {
		return nil, err
	}
	return &costProgram{prg: prg}, nil
}

type costProgram struct {
	prg cel.Program
}

var _ cel.Program = (*costProgram)(nil)

func (p *costProgram) Eval(vars any) (ref.Val, *cel.EvalDetails, error) {
	return p.ContextEval(context.Background(), vars)
}

// ConcurrentEval charges the same way ContextEval does. Forwarding the inner channel
// straight through would leave a method on the chokepoint that accounts for nothing, so a
// future caller could evaluate without ever touching the request budget. Nothing in this
// repository calls it today; it is implemented so the chokepoint's guarantee holds for
// anything that does.
func (p *costProgram) ConcurrentEval(ctx context.Context, vars any) <-chan cel.EvalResult {
	in := p.prg.ConcurrentEval(ctx, vars)
	out := make(chan cel.EvalResult, 1)
	go func() {
		defer close(out)
		for res := range in {
			res.Val, res.EvalDetails, res.Err = p.account(ctx, res.Val, res.EvalDetails, res.Err)
			out <- res
		}
	}()
	return out
}

// ContextEval is the one evaluation path every Kyverno program is expected to go through.
func (p *costProgram) ContextEval(ctx context.Context, vars any) (ref.Val, *cel.EvalDetails, error) {
	val, details, err := p.prg.ContextEval(ctx, vars)
	return p.account(ctx, val, details, err)
}

// account translates cel-go's own cost cancellation into this package's sentinel and charges
// the request budget. Shared by ContextEval and ConcurrentEval so neither can drift into
// evaluating without charging.
func (p *costProgram) account(ctx context.Context, val ref.Val, details *cel.EvalDetails, err error) (ref.Val, *cel.EvalDetails, error) {
	var cost uint64
	if details != nil {
		if c := details.ActualCost(); c != nil {
			cost = *c
		}
	}
	if isCostLimitError(err) {
		// An aborted program must never be charged less than a completed one would have
		// been: cel-go's own cost cancellation can fire with ActualCost() still below L
		// (the panic unwinds before the final Observe), so floor the charge at the limit.
		if limit := perProgramLimit(); cost < limit {
			cost = limit
		}
		// A cost-limit refusal otherwise only ever surfaces in the admission/report
		// response, never in the controller's own log -- undiagnosable in production.
		klog.V(4).InfoS("cel: per-program cost limit exceeded", "cost", cost, "limit", perProgramLimit())
		err = fmt.Errorf("%w: %v", ErrCostLimitExceeded, err)
	}
	// requestbudget.Charge degrades gracefully (see its own doc comment): a ctx with no
	// budget -- an entry point this cut did not wire -- charges nothing and never refuses,
	// leaving today's per-program-limit-only behavior in place rather than failing the call.
	if chargeErr := requestbudget.Charge(ctx, cost); chargeErr != nil && err == nil {
		err = chargeErr
	}
	// cel-go's own recover() turns a panic inside evaluation (e.g. a cel-go interpreter
	// bug, not a resource refusal from the cost limit or a guard) into an error whose text
	// starts with "internal error:", with no controller log line anywhere else on this
	// path. IsResourceError is everything this package expects to refuse legitimately;
	// anything else reaching here is worth a log line to diagnose.
	if err != nil && !IsResourceError(err) && strings.Contains(err.Error(), "internal error") {
		klog.ErrorS(err, "cel: evaluation returned an internal error, not a resource refusal")
	}
	return val, details, err
}

// isCostLimitError reports whether err is cel-go's own per-program cost-limit
// cancellation (interpreter.EvalCancelledError{Cause: CostLimitExceeded}, raised as a
// panic inside Observe and recovered by cel-go's own Eval/ContextEval, cel/program.go).
func isCostLimitError(err error) bool {
	var cancelled interpreter.EvalCancelledError
	if errors.As(err, &cancelled) {
		return cancelled.Cause == interpreter.CostLimitExceeded
	}
	return false
}
