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
