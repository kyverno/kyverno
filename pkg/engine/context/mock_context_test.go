package context

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockContext_Query_JMESPathMultiSelectExpression(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*", "static*")

	_, err := ctx.Query("[allPods, static] []")
	assert.NoError(t, err)
}

func TestMockContext_Query_JMESPathMultiSelectExpression_WithSpaces(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*", "static*")

	_, err := ctx.Query(" [allPods, static] [] ")
	assert.NoError(t, err)
}

func TestMockContext_Query_JMESPathMultiSelectExpression_InvalidVariable(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*")

	_, err := ctx.Query("[allPods, static] []")
	assert.Error(t, err)
	var invalidErr InvalidVariableError
	assert.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "static")
	assert.NotContains(t, err.Error(), "[allPods, static] []")
}

func TestMockContext_Query_JMESPathMultiSelectExpression_SingleIdentifier(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*")

	_, err := ctx.Query("[allPods]")
	assert.NoError(t, err)
}

func TestMockContext_Query_JMESPathMultiSelectExpression_EmptyBrackets(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*")

	_, err := ctx.Query("[]")
	assert.Error(t, err)
}

func TestMockContext_Query_JMESPathMultiSelectExpression_NestedBrackets(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*", "static*")

	// "allPods[]" is a valid list member (flatten of allPods) and must not be excluded
	// just because the multi-select element itself contains brackets.
	_, err := ctx.Query("[allPods[], static]")
	assert.NoError(t, err)
}

func TestMockContext_Query_JMESPathMultiSelectExpression_NestedMultiSelect(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*", "static*")

	// A multi-select list nested inside another multi-select list must have its own
	// elements validated too.
	_, err := ctx.Query("[[allPods, static], allPods]")
	assert.NoError(t, err)
}

func TestMockContext_Query_JMESPathMultiSelectExpression_NestedMultiSelect_InvalidVariable(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*", "static*")

	_, err := ctx.Query("[[allPods, typo], allPods]")
	assert.Error(t, err)
	var invalidErr InvalidVariableError
	assert.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "typo")
}

func TestMockContext_Query_JMESPathMultiSelectExpression_PipedSuffix_InvalidVariable(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*", "static*")

	// A trailing expression after a pipe must be validated too -- it is not enough for
	// the multi-select list's own elements to be allowed.
	_, err := ctx.Query("[allPods, static] | forbidden")
	assert.Error(t, err)
	var invalidErr InvalidVariableError
	assert.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "variable forbidden")
}

func TestMockContext_Query_JMESPathMultiSelectExpression_RegexShortCircuit_InvalidVariable(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re)

	// Without the fix, the whole-query regex check (ctx.re matches "request\.") would run
	// before the multi-select list is decomposed, letting "typo" slip through because the
	// substring "request." is present anywhere in the raw query.
	_, err := ctx.Query("[request.object, typo]")
	assert.Error(t, err)
	var invalidErr InvalidVariableError
	assert.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "typo")
}

// --- round 3: an allowed identifier masking a forbidden one via whole-fragment
// MatchString, e.g. "request."/"length(" matching and letting "typo" through.

func TestMockContext_Query_JMESPathMultiSelectExpression_OrExpression_InvalidVariable(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re)

	// "request." matching the regex as a substring of the whole fragment must not mask
	// the forbidden "typo" on the other side of the "||".
	_, err := ctx.Query("[request.object || typo]")
	assert.Error(t, err)
	var invalidErr InvalidVariableError
	assert.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "typo")
}

func TestMockContext_Query_JMESPathMultiSelectExpression_FunctionArgument_InvalidVariable(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*")

	// The piped suffix "length(typo)" must have its argument validated instead of being
	// accepted outright because the fragment "looks like" a function call.
	_, err := ctx.Query("[allPods] | length(typo)")
	assert.Error(t, err)
	var invalidErr InvalidVariableError
	assert.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "typo")
}

// --- round 3: a JMESPath literal inside a multi-select element must not be treated as
// a context-variable identifier and run through the allow-list.

func TestMockContext_Query_JMESPathMultiSelectExpression_StringLiteral(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*")

	// 'static' is a quoted-string JMESPath literal, not a reference to a "static"
	// context variable -- it must be accepted even though no "static*" pattern (nor
	// anything in re) was registered.
	_, err := ctx.Query("[allPods, 'static']")
	assert.NoError(t, err)
}

func TestMockContext_Query_JMESPathMultiSelectExpression_RawStringLiteral(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*")

	// A backtick raw/JSON literal element must likewise be skipped.
	_, err := ctx.Query("[allPods, `\"static\"`]")
	assert.NoError(t, err)
}

// --- round 3: "||" must not be mistaken for a "|" pipe when validating whatever
// follows a multi-select list.

func TestMockContext_Query_JMESPathMultiSelectExpression_OrSuffix_Literal(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*", "static*")

	// Trimming a single leading "|" off of "|| `[]`" would turn it into "| `[]`" and
	// then try to validate "`[]`" as if it were a piped variable reference, which is
	// wrong on two counts: it's a "||" (or) suffix, not a "|" (pipe) suffix, and its
	// right-hand side is a literal, not an identifier.
	_, err := ctx.Query("[allPods, static] || `[]`")
	assert.NoError(t, err)
}

func TestMockContext_Query_JMESPathMultiSelectExpression_OrSuffix_InvalidVariable(t *testing.T) {
	re := regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	ctx := NewMockContext(re, "allPods*", "static*")

	// The right-hand side of a genuine "||" suffix must still be validated when it is
	// itself an identifier.
	_, err := ctx.Query("[allPods, static] || forbidden")
	assert.Error(t, err)
	var invalidErr InvalidVariableError
	assert.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "variable forbidden")
}
