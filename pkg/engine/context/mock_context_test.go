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
