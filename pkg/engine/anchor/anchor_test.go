package anchor

import (
	"testing"

	"gotest.tools/assert"
)

func TestWrappedWithParentheses_StringIsWrappedWithParentheses(t *testing.T) {
	str := "(something)"
	anchor := Parse(str)
	assert.Assert(t, IsCondition(anchor))
}

func TestWrappedWithParentheses_StringHasOnlyParentheses(t *testing.T) {
	str := "()"
	anchor := Parse(str)
	assert.Assert(t, !IsCondition(anchor))
}

func TestWrappedWithParentheses_StringHasNoParentheses(t *testing.T) {
	str := "something"
	anchor := Parse(str)
	assert.Assert(t, !IsCondition(anchor))
}

func TestWrappedWithParentheses_StringHasLeftParentheses(t *testing.T) {
	str := "(something"
	anchor := Parse(str)
	assert.Assert(t, !IsCondition(anchor))
}

func TestWrappedWithParentheses_StringHasRightParentheses(t *testing.T) {
	str := "something)"
	anchor := Parse(str)
	assert.Assert(t, !IsCondition(anchor))
}

func TestWrappedWithParentheses_StringParenthesesInside(t *testing.T) {
	str := "so)m(et(hin)g"
	anchor := Parse(str)
	assert.Assert(t, !IsCondition(anchor))
}

func TestWrappedWithParentheses_Empty(t *testing.T) {
	str := ""
	anchor := Parse(str)
	assert.Assert(t, !IsCondition(anchor))
}

func TestIsExistence_Yes(t *testing.T) {
	anchor := Parse("^(abc)")
	assert.Assert(t, IsExistence(anchor))
}

func TestIsExistence_NoRightBracket(t *testing.T) {
	anchor := Parse("^(abc")
	assert.Assert(t, !IsExistence(anchor))
}

func TestIsExistence_OnlyHat(t *testing.T) {
	anchor := Parse("^abc")
	assert.Assert(t, !IsExistence(anchor))
}

func TestIsExistence_Condition(t *testing.T) {
	anchor := Parse("(abc)")
	assert.Assert(t, !IsExistence(anchor))
}

func TestRemoveAnchorsFromPath_WorksWithAbsolutePath(t *testing.T) {
	newPath := RemoveAnchorsFromPath("/path/(to)/X(anchors)")
	assert.Equal(t, newPath, "/path/to/anchors")
}

func TestRemoveAnchorsFromPath_WorksWithRelativePath(t *testing.T) {
	newPath := RemoveAnchorsFromPath("path/(to)/X(anchors)")
	assert.Equal(t, newPath, "path/to/anchors")
}

func TestIsEqualityAnchor_Yes(t *testing.T) {
	anchor := Parse("=(abc)")
	assert.Assert(t, IsEquality(anchor))
}

func TestIsEquality_NoRightBracket(t *testing.T) {
	anchor := Parse("=(abc")
	assert.Assert(t, !IsEquality(anchor))
}

func TestIsEquality_OnlyHat(t *testing.T) {
	anchor := Parse("=abc")
	assert.Assert(t, !IsEquality(anchor))
}

func TestIsAddition_Yes(t *testing.T) {
	anchor := Parse("+(abc)")
	assert.Assert(t, IsAddIfNotPresent(anchor))
}

func TestIsAddition_NoRightBracket(t *testing.T) {
	anchor := Parse("+(abc")
	assert.Assert(t, !IsAddIfNotPresent(anchor))
}

func TestIsAddition_OnlyHat(t *testing.T) {
	anchor := Parse("+abc")
	assert.Assert(t, !IsAddIfNotPresent(anchor))
}

func TestIsNegation_Yes(t *testing.T) {
	anchor := Parse("X(abc)")
	assert.Assert(t, IsNegation(anchor))
}

func TestIsNegation_NoRightBracket(t *testing.T) {
	anchor := Parse("X(abc")
	assert.Assert(t, !IsNegation(anchor))
}

func TestIsNegation_OnlyHat(t *testing.T) {
	anchor := Parse("Xabc")
	assert.Assert(t, !IsNegation(anchor))
}

func TestIsGlobal_Yes(t *testing.T) {
	anchor := Parse("<(abc)")
	assert.Assert(t, IsGlobal(anchor))
}

func TestIsGlobal_NoRightBracket(t *testing.T) {
	anchor := Parse("<(abc")
	assert.Assert(t, !IsGlobal(anchor))
}

func TestIsGlobal_OnlyHat(t *testing.T) {
	anchor := Parse("<abc")
	assert.Assert(t, !IsGlobal(anchor))
}
func TestIsConditionAnchor_Yes(t *testing.T) {
	anchor := Parse("(abc)")
	assert.Assert(t, IsCondition(anchor))
}

func TestIsConditionAnchor_NoRightBracket(t *testing.T) {
	anchor := Parse("(abc")
	assert.Assert(t, !IsCondition(anchor))
}

func TestIsConditionAnchor_Onlytext(t *testing.T) {
	anchor := Parse("abc")
	assert.Assert(t, !IsCondition(anchor))
}
