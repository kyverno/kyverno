package anchor

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

func TestRemoveAnchorsFromPath_WorksWithAbsolutePath(t *testing.T) {
	newPath := RemoveAnchorsFromPath("/path/(to)/X(anchors)")
	assert.Equal(t, newPath, "/path/to/anchors")
}

func TestRemoveAnchorsFromPath_WorksWithRelativePath(t *testing.T) {
	newPath := RemoveAnchorsFromPath("path/(to)/X(anchors)")
	assert.Equal(t, newPath, "path/to/anchors")
}

func TestRemoveAnchorsFromPath_WorksWithIncompleteRelativePath(t *testing.T) {
	newPath := RemoveAnchorsFromPath("path/(t(o)/X(anchors)")
	assert.Equal(t, newPath, "path/t(o/anchors")
}

func TestIsEqualityAnchor_Yes(t *testing.T) {
	assert.Assert(t, IsEqualityAnchor("=(abc)"))
}

func TestIsEqualityAnchor_NoRightBracket(t *testing.T) {
	assert.Assert(t, !IsEqualityAnchor("=(abc"))
}

func TestIsEqualityAnchor_OnlyHat(t *testing.T) {
	assert.Assert(t, !IsEqualityAnchor("=abc"))
}

func TestIsAdditionAnchor_Yes(t *testing.T) {
	assert.Assert(t, IsAddIfNotPresentAnchor("+(abc)"))
}

func TestIsAdditionAnchor_NoRightBracket(t *testing.T) {
	assert.Assert(t, !IsAddIfNotPresentAnchor("+(abc"))
}

func TestIsAdditionAnchor_OnlyHat(t *testing.T) {
	assert.Assert(t, !IsAddIfNotPresentAnchor("+abc"))
}

func TestIsNegationAnchor_Yes(t *testing.T) {
	assert.Assert(t, IsNegationAnchor("X(abc)"))
}

func TestIsNegationAnchor_NoRightBracket(t *testing.T) {
	assert.Assert(t, !IsNegationAnchor("X(abc"))
}

func TestIsNegationAnchor_OnlyHat(t *testing.T) {
	assert.Assert(t, !IsNegationAnchor("Xabc"))
}

func TestIsGlobalAnchor_Yes(t *testing.T) {
	assert.Assert(t, IsGlobalAnchor("<(abc)"))
}

func TestIsGlobalAnchor_NoRightBracket(t *testing.T) {
	assert.Assert(t, !IsGlobalAnchor("<(abc"))
}

func TestIsGlobalAnchor_OnlyHat(t *testing.T) {
	assert.Assert(t, !IsGlobalAnchor("<abc"))
}

func TestIsConditionAnchor_Yes(t *testing.T) {
	assert.Assert(t, IsConditionAnchor("(abc)"))
}

func TestIsConditionAnchor_NoRightBracket(t *testing.T) {
	assert.Assert(t, !IsConditionAnchor("(abc"))
}

func TestIsConditionAnchor_OnlyHat(t *testing.T) {
	assert.Assert(t, !IsConditionAnchor("abc"))
}
