package common

import (
	"testing"

	"gotest.tools/assert"
)

func TestWrappedWithParentheses_StringIsWrappedWithParentheses(t *testing.T) {
	str := "(something)"
	assert.Assert(t, IsConditionAnchor(str))
}

func TestWrappedWithParentheses_StringHasOnlyParentheses(t *testing.T) {
	str := "()"
	assert.Assert(t, IsConditionAnchor(str))
}

func TestWrappedWithParentheses_StringHasNoParentheses(t *testing.T) {
	str := "something"
	assert.Assert(t, !IsConditionAnchor(str))
}

func TestWrappedWithParentheses_StringHasLeftParentheses(t *testing.T) {
	str := "(something"
	assert.Assert(t, !IsConditionAnchor(str))
}

func TestWrappedWithParentheses_StringHasRightParentheses(t *testing.T) {
	str := "something)"
	assert.Assert(t, !IsConditionAnchor(str))
}

func TestWrappedWithParentheses_StringParenthesesInside(t *testing.T) {
	str := "so)m(et(hin)g"
	assert.Assert(t, !IsConditionAnchor(str))
}

func TestWrappedWithParentheses_Empty(t *testing.T) {
	str := ""
	assert.Assert(t, !IsConditionAnchor(str))
}

func TestIsExistenceAnchor_Yes(t *testing.T) {
	assert.Assert(t, IsExistenceAnchor("^(abc)"))
}

func TestIsExistenceAnchor_NoRightBracket(t *testing.T) {
	assert.Assert(t, !IsExistenceAnchor("^(abc"))
}

func TestIsExistenceAnchor_OnlyHat(t *testing.T) {
	assert.Assert(t, !IsExistenceAnchor("^abc"))
}

func TestIsExistenceAnchor_ConditionAnchor(t *testing.T) {
	assert.Assert(t, !IsExistenceAnchor("(abc)"))
}
