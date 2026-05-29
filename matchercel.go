package mercure

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/cel-go/cel"
)

// errCELMustReturnBool is returned when a CEL expression does not return a boolean.
var errCELMustReturnBool = errors.New("CEL expression must return bool")

// celEvaluationCostLimit caps runtime evaluation cost per CEL call to prevent
// DoS from pathological expressions submitted by untrusted JWTs (e.g.
// `topics.all(x, topics.all(y, ...))` style fan-out). The value is deliberately
// generous for legitimate matchers but refuses expressions that would spend
// multiple milliseconds of CPU per evaluation.
const celEvaluationCostLimit uint64 = 1_000_000

// NewCELMatcher returns a CEL (Common Expression Language) matcher. CEL
// expressions receive a `topics` variable (list of strings) and must return
// a boolean. Compilation failures are logged via the given logger; a nil
// logger falls back to slog.Default(). Each instance keeps its own
// compiled-program cache, so callers that want isolated caches (e.g. per
// Caddy module instance) can construct multiple matchers.
//
// Unlike the other built-in matchers, CEL is exposed only through this
// constructor: it carries a logger and a per-instance compile cache, so
// sharing a process-wide singleton would mix configurations across hubs.
func NewCELMatcher(logger *slog.Logger) Matcher { //nolint:ireturn
	return newCELMatcherType(logger)
}

// celCompiled holds the outcome of a single CEL compilation attempt — either
// a ready program or the error that caused compilation to fail. Caching the
// error path avoids re-running cel-go's parser on every Match call for
// expressions that a malicious JWT author knows are invalid.
type celCompiled struct {
	program cel.Program
	err     error
}

type celMatcherType struct {
	env      *cel.Env
	logger   *slog.Logger
	programs sync.Map // pattern → *celCompiled
}

func newCELMatcherType(logger *slog.Logger) *celMatcherType {
	if logger == nil {
		logger = slog.Default()
	}

	env, err := cel.NewEnv(
		cel.Variable("topics", cel.ListType(cel.StringType)),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create CEL environment: %v", err))
	}

	return &celMatcherType{env: env, logger: logger}
}

// Match evaluates the CEL expression with the given topics array.
func (c *celMatcherType) Match(topics []string, pattern string) bool {
	prg, err := c.getOrCompile(pattern)
	if err != nil {
		return false
	}

	out, _, err := prg.Eval(map[string]any{
		"topics": topics,
	})
	if err != nil {
		return false
	}

	result, ok := out.Value().(bool)

	return ok && result
}

func (c *celMatcherType) getOrCompile(pattern string) (cel.Program, error) { //nolint:ireturn
	if cached, ok := c.programs.Load(pattern); ok {
		entry := cached.(*celCompiled)

		return entry.program, entry.err
	}

	prg, err := c.compile(pattern)
	entry := &celCompiled{program: prg, err: err}

	actual, loaded := c.programs.LoadOrStore(pattern, entry)
	if !loaded && err != nil {
		// Log once per pattern so operators notice malformed expressions.
		c.logger.Warn("CEL matcher compilation failed, pattern will be rejected on all future calls", "pattern", pattern, "error", err)
	}

	final := actual.(*celCompiled)

	return final.program, final.err
}

func (c *celMatcherType) compile(expression string) (cel.Program, error) { //nolint:ireturn
	ast, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation error: %w", issues.Err())
	}

	// Verify the expression returns a bool
	if ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("%w, got %s", errCELMustReturnBool, ast.OutputType())
	}

	prg, err := c.env.Program(ast, cel.CostLimit(celEvaluationCostLimit))
	if err != nil {
		return nil, fmt.Errorf("CEL program creation error: %w", err)
	}

	return prg, nil
}
