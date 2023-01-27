package anchor

import (
	"testing"

	"gotest.tools/assert"
)

func TestWrappedWithParentheses_StringIsWrappedWithParentheses(t *testing.T) {
	str := "(something)"
	anchor := Parse(str)
	assert.Assert(t, anchor.IsCondition())
}

func TestWrappedWithParentheses_StringHasOnlyParentheses(t *testing.T) {
	str := "()"
	anchor := Parse(str)
	assert.Assert(t, anchor.IsCondition())
}

func TestWrappedWithParentheses_StringHasNoParentheses(t *testing.T) {
	str := "something"
	anchor := Parse(str)
	assert.Assert(t, !anchor.IsCondition())
}

func TestWrappedWithParentheses_StringHasLeftParentheses(t *testing.T) {
	str := "(something"
	anchor := Parse(str)
	assert.Assert(t, !anchor.IsCondition())
}

func TestWrappedWithParentheses_StringHasRightParentheses(t *testing.T) {
	str := "something)"
	anchor := Parse(str)
	assert.Assert(t, !anchor.IsCondition())
}

func TestWrappedWithParentheses_StringParenthesesInside(t *testing.T) {
	str := "so)m(et(hin)g"
	anchor := Parse(str)
	assert.Assert(t, !anchor.IsCondition())
}

func TestWrappedWithParentheses_Empty(t *testing.T) {
	str := ""
	anchor := Parse(str)
	assert.Assert(t, !anchor.IsCondition())
}

func TestIsExistence_Yes(t *testing.T) {
	anchor := Parse("^(abc)")
	assert.Assert(t, anchor.IsExistence())
}

func TestIsExistence_NoRightBracket(t *testing.T) {
	anchor := Parse("^(abc")
	assert.Assert(t, !anchor.IsExistence())
}

func TestIsExistence_OnlyHat(t *testing.T) {
	anchor := Parse("^abc")
	assert.Assert(t, !anchor.IsExistence())
}

func TestIsExistence_Condition(t *testing.T) {
	anchor := Parse("(abc)")
	assert.Assert(t, !anchor.IsExistence())
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
	assert.Assert(t, anchor.IsEquality())
}

func TestIsEquality_NoRightBracket(t *testing.T) {
	anchor := Parse("=(abc")
	assert.Assert(t, !anchor.IsEquality())
}

func TestIsEquality_OnlyHat(t *testing.T) {
	anchor := Parse("=abc")
	assert.Assert(t, !anchor.IsEquality())
}

func TestIsAddition_Yes(t *testing.T) {
	anchor := Parse("+(abc)")
	assert.Assert(t, anchor.IsAddIfNotPresent())
}

func TestIsAddition_NoRightBracket(t *testing.T) {
	anchor := Parse("+(abc")
	assert.Assert(t, !anchor.IsAddIfNotPresent())
}

func TestIsAddition_OnlyHat(t *testing.T) {
	anchor := Parse("+abc")
	assert.Assert(t, !anchor.IsAddIfNotPresent())
}

func TestIsNegation_Yes(t *testing.T) {
	anchor := Parse("X(abc)")
	assert.Assert(t, anchor.IsNegation())
}

func TestIsNegation_NoRightBracket(t *testing.T) {
	anchor := Parse("X(abc")
	assert.Assert(t, !anchor.IsNegation())
}

func TestIsNegation_OnlyHat(t *testing.T) {
	anchor := Parse("Xabc")
	assert.Assert(t, !anchor.IsNegation())
}

func TestIsGlobal_Yes(t *testing.T) {
	anchor := Parse("<(abc)")
	assert.Assert(t, anchor.IsGlobal())
}

func TestIsGlobal_NoRightBracket(t *testing.T) {
	anchor := Parse("<(abc")
	assert.Assert(t, !anchor.IsGlobal())
}

func TestIsGlobal_OnlyHat(t *testing.T) {
	anchor := Parse("<abc")
	assert.Assert(t, !anchor.IsGlobal())
}
func TestIsConditionAnchor_Yes(t *testing.T) {
	anchor := Parse("(abc)")
	assert.Assert(t, anchor.IsCondition())
}

func TestIsConditionAnchor_NoRightBracket(t *testing.T) {
	anchor := Parse("(abc")
	assert.Assert(t, !anchor.IsCondition())
}

func TestIsConditionAnchor_Onlytext(t *testing.T) {
	anchor := Parse("abc")
	assert.Assert(t, !anchor.IsCondition())
}
