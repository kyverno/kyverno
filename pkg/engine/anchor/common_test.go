package anchor

import (
	"testing"

	"gotest.tools/assert"
)

func TestWrappedWithParentheses_StringIsWrappedWithParentheses(t *testing.T) {
	str := "(something)"
	anchor := ParseAnchor(str)
	assert.Assert(t, anchor.IsConditionAnchor())
}

func TestWrappedWithParentheses_StringHasOnlyParentheses(t *testing.T) {
	str := "()"
	anchor := ParseAnchor(str)
	assert.Assert(t, anchor.IsConditionAnchor())
}

func TestWrappedWithParentheses_StringHasNoParentheses(t *testing.T) {
	str := "something"
	anchor := ParseAnchor(str)
	assert.Assert(t, !anchor.IsConditionAnchor())
}

func TestWrappedWithParentheses_StringHasLeftParentheses(t *testing.T) {
	str := "(something"
	anchor := ParseAnchor(str)
	assert.Assert(t, !anchor.IsConditionAnchor())
}

func TestWrappedWithParentheses_StringHasRightParentheses(t *testing.T) {
	str := "something)"
	anchor := ParseAnchor(str)
	assert.Assert(t, !anchor.IsConditionAnchor())
}

func TestWrappedWithParentheses_StringParenthesesInside(t *testing.T) {
	str := "so)m(et(hin)g"
	anchor := ParseAnchor(str)
	assert.Assert(t, !anchor.IsConditionAnchor())
}

func TestWrappedWithParentheses_Empty(t *testing.T) {
	str := ""
	anchor := ParseAnchor(str)
	assert.Assert(t, !anchor.IsConditionAnchor())
}

func TestIsExistenceAnchor_Yes(t *testing.T) {
	anchor := ParseAnchor("^(abc)")
	assert.Assert(t, anchor.IsExistenceAnchor())
}

func TestIsExistenceAnchor_NoRightBracket(t *testing.T) {
	anchor := ParseAnchor("^(abc")
	assert.Assert(t, !anchor.IsExistenceAnchor())
}

func TestIsExistenceAnchor_OnlyHat(t *testing.T) {
	anchor := ParseAnchor("^abc")
	assert.Assert(t, !anchor.IsExistenceAnchor())
}

func TestIsExistenceAnchor_ConditionAnchor(t *testing.T) {
	anchor := ParseAnchor("(abc)")
	assert.Assert(t, !anchor.IsExistenceAnchor())
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
	anchor := ParseAnchor("=(abc)")
	assert.Assert(t, anchor.IsEqualityAnchor())
}

func TestIsEqualityAnchor_NoRightBracket(t *testing.T) {
	anchor := ParseAnchor("=(abc")
	assert.Assert(t, !anchor.IsEqualityAnchor())
}

func TestIsEqualityAnchor_OnlyHat(t *testing.T) {
	anchor := ParseAnchor("=abc")
	assert.Assert(t, !anchor.IsEqualityAnchor())
}

func TestIsAdditionAnchor_Yes(t *testing.T) {
	anchor := ParseAnchor("+(abc)")
	assert.Assert(t, anchor.IsAddIfNotPresentAnchor())
}

func TestIsAdditionAnchor_NoRightBracket(t *testing.T) {
	anchor := ParseAnchor("+(abc")
	assert.Assert(t, !anchor.IsAddIfNotPresentAnchor())
}

func TestIsAdditionAnchor_OnlyHat(t *testing.T) {
	anchor := ParseAnchor("+abc")
	assert.Assert(t, !anchor.IsAddIfNotPresentAnchor())
}

func TestIsNegationAnchor_Yes(t *testing.T) {
	anchor := ParseAnchor("X(abc)")
	assert.Assert(t, anchor.IsNegationAnchor())
}

func TestIsNegationAnchor_NoRightBracket(t *testing.T) {
	anchor := ParseAnchor("X(abc")
	assert.Assert(t, !anchor.IsNegationAnchor())
}

func TestIsNegationAnchor_OnlyHat(t *testing.T) {
	anchor := ParseAnchor("Xabc")
	assert.Assert(t, !anchor.IsNegationAnchor())
}

func TestIsGlobalAnchor_Yes(t *testing.T) {
	anchor := ParseAnchor("<(abc)")
	assert.Assert(t, anchor.IsGlobalAnchor())
}

func TestIsGlobalAnchor_NoRightBracket(t *testing.T) {
	anchor := ParseAnchor("<(abc")
	assert.Assert(t, !anchor.IsGlobalAnchor())
}

func TestIsGlobalAnchor_OnlyHat(t *testing.T) {
	anchor := ParseAnchor("<abc")
	assert.Assert(t, !anchor.IsGlobalAnchor())
}
func TestIsConditionAnchor_Yes(t *testing.T) {
	anchor := ParseAnchor("(abc)")
	assert.Assert(t, anchor.IsConditionAnchor())
}

func TestIsConditionAnchor_NoRightBracket(t *testing.T) {
	anchor := ParseAnchor("(abc")
	assert.Assert(t, !anchor.IsConditionAnchor())
}

func TestIsConditionAnchor_Onlytext(t *testing.T) {
	anchor := ParseAnchor("abc")
	assert.Assert(t, !anchor.IsConditionAnchor())
}
