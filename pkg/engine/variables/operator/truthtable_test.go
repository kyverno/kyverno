package operator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

// TestBothFalseContradictionResolved pins the fix for the shape where one
// non-string element anywhere in a []interface{} value array used to
// collapse both In and NotIn to false, even when a key element was
// literally present as a matching string.
func TestBothFalseContradictionResolved(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	notin := NewNotInHandler(logr.Discard(), nil)

	tests := []struct {
		name       string
		key        []interface{}
		value      []interface{}
		expectIn   bool
		expectNoIn bool
	}{
		{
			name:       "integer value array",
			key:        []interface{}{"1"},
			value:      []interface{}{1, 2, 3},
			expectIn:   true,
			expectNoIn: false,
		},
		{
			name:       "mixed-type value array with the key present as a string",
			key:        []interface{}{"1"},
			value:      []interface{}{"1", 2, true},
			expectIn:   true,
			expectNoIn: false,
		},
		{
			name:       "opaque element in the value array does not poison the comparison",
			key:        []interface{}{"1"},
			value:      []interface{}{"1", map[string]interface{}{"a": "b"}},
			expectIn:   true,
			expectNoIn: false,
		},
		{
			name:       "bool value array",
			key:        []interface{}{"true"},
			value:      []interface{}{true, false},
			expectIn:   true,
			expectNoIn: false,
		},
		{
			name:       "key absent still resolves to the complement",
			key:        []interface{}{"9"},
			value:      []interface{}{1, 2, 3},
			expectIn:   false,
			expectNoIn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectIn, in.Evaluate(tt.key, tt.value))
			assert.Equal(t, tt.expectNoIn, notin.Evaluate(tt.key, tt.value))
		})
	}
}

// TestBothTrueContradictionResolved pins the fix for setExistsInArray's
// string-value shortcut, which used to ignore the notIn flag so that both
// In and NotIn returned true for the same single-key-vs-string-value shape.
func TestBothTrueContradictionResolved(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	notin := NewNotInHandler(logr.Discard(), nil)

	key := []interface{}{"1"}
	value := "1"

	assert.True(t, in.Evaluate(key, value))
	assert.False(t, notin.Evaluate(key, value))
}

// TestNonJSONStringValueIsNoLongerBothFalse pins the string-value grammar
// harmonization: a non-JSON string value used to error out (invalidType)
// for In/NotIn, producing a both-false result even when it should be a
// well-defined miss/complement pair.
func TestNonJSONStringValueIsNoLongerBothFalse(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	notin := NewNotInHandler(logr.Discard(), nil)

	key := []interface{}{"x"}
	value := "y"

	assert.False(t, in.Evaluate(key, value))
	assert.True(t, notin.Evaluate(key, value))
}

// TestRealisticPortsComparisonWorks is the motivating example: a native
// key array of ports and a native value array of ports, both JSON numbers,
// must compare successfully instead of silently no-op-ing.
func TestRealisticPortsComparisonWorks(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	notin := NewNotInHandler(logr.Discard(), nil)

	key := []interface{}{float64(80), float64(443)}
	value := []interface{}{float64(80), float64(443)}

	assert.True(t, in.Evaluate(key, value))
	assert.False(t, notin.Evaluate(key, value))
}

// TestExactVsWildcardSplitStaysPinned confirms axis 1 (element typing) was
// unified without erasing axis 2 (exact equality vs symmetric wildcard):
// the scalar-key path still honors wildcard values, while the array-key
// (set) path stays exact. Erasing this split would be a silent fail-open
// for pattern-shaped values in exclude position.
func TestExactVsWildcardSplitStaysPinned(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)

	assert.True(t, in.Evaluate("1.1.1.1", []interface{}{"1*"}))
	assert.False(t, in.Evaluate([]interface{}{"x"}, []interface{}{"*"}))
}

// TestCompositeCarveOutPreservedAcrossFamily confirms the universal-pattern
// carve-out for opaque elements still holds after the primitive rewiring:
// a "*"/"**" value keeps matching a composite element on wildcard paths,
// but never on the exact-equality path (In/NotIn's array-key set path).
func TestCompositeCarveOutPreservedAcrossFamily(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	notin := NewNotInHandler(logr.Discard(), nil)
	anyin := NewAnyInHandler(logr.Discard(), nil)
	allin := NewAllInHandler(logr.Discard(), nil)

	composite := []interface{}{map[string]interface{}{"a": "b"}}

	assert.True(t, anyin.Evaluate(composite, []interface{}{"*"}))
	assert.True(t, allin.Evaluate(composite, []interface{}{"**"}))
	assert.False(t, anyin.Evaluate(composite, []interface{}{"prod*"}))

	// In/NotIn's array-key path is exact equality and never honored
	// wildcards, carve-out or not.
	assert.False(t, in.Evaluate(composite, []interface{}{"*"}))
	assert.True(t, notin.Evaluate(composite, []interface{}{"*"}))
}

// TestInAndNotInAreComplements pins NotIn == !In across scalar, mixed, and
// composite element shapes -- the property the both-false and both-true
// contradictions each violated.
func TestInAndNotInAreComplements(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	notin := NewNotInHandler(logr.Discard(), nil)

	cases := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{"all-string arrays", []interface{}{"a", "b"}, []interface{}{"a", "b", "c"}},
		{"integer value array", []interface{}{"1"}, []interface{}{1, 2, 3}},
		{"mixed value array", []interface{}{"1", "9"}, []interface{}{"1", 2, true}},
		{"opaque key element", []interface{}{"1", map[string]interface{}{"a": "b"}}, []interface{}{1, 2}},
		{"opaque value element", []interface{}{"1"}, []interface{}{1, map[string]interface{}{"a": "b"}}},
		{"single string value", []interface{}{"1"}, "1"},
		{"non-JSON string value", []interface{}{"x"}, "y"},
		{"universal pattern vs composite key", []interface{}{map[string]interface{}{"a": "b"}}, []interface{}{"*"}},
		{"scalar key with wildcard value", "1.1.1.1", []interface{}{"1*"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, in.Evaluate(c.key, c.value), !notin.Evaluate(c.key, c.value))
		})
	}
}

// TestAnyNotInComplementsAllIn and TestAllNotInComplementsAnyIn pin the
// same structural-negation property for the Any/All twins, including the
// carve-out rows.
func TestAnyNotInComplementsAllIn(t *testing.T) {
	allin := NewAllInHandler(logr.Discard(), nil)
	anynotin := NewAnyNotInHandler(logr.Discard(), nil)

	cases := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{"all-string arrays", []interface{}{"a", "b"}, []interface{}{"a", "b", "c"}},
		{"integer value array", []interface{}{"1"}, []interface{}{1, 2, 3}},
		{"opaque key element vs universal pattern", []interface{}{map[string]interface{}{"a": "b"}, "x"}, []interface{}{"*"}},
		{"opaque key element vs literal pattern", []interface{}{map[string]interface{}{"a": "b"}}, []interface{}{"prod*"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, allin.Evaluate(c.key, c.value), !anynotin.Evaluate(c.key, c.value))
		})
	}
}

func TestAllNotInComplementsAnyIn(t *testing.T) {
	anyin := NewAnyInHandler(logr.Discard(), nil)
	allnotin := NewAllNotInHandler(logr.Discard(), nil)

	cases := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{"all-string arrays", []interface{}{"a", "d"}, []interface{}{"a", "b", "c"}},
		{"integer value array", []interface{}{"1", "9"}, []interface{}{1, 2, 3}},
		{"opaque key element vs universal pattern", []interface{}{map[string]interface{}{"a": "b"}, "x"}, []interface{}{"*"}},
		{"opaque key element vs literal pattern", []interface{}{map[string]interface{}{"a": "b"}}, []interface{}{"prod*"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, anyin.Evaluate(c.key, c.value), !allnotin.Evaluate(c.key, c.value))
		})
	}
}

// TestBareScalarValueComparesInsteadOfErroring pins the fix for the gap
// D2' left behind: a "value" operand that is a bare comparable scalar
// (not wrapped in an array, not a string) used to be rejected as an
// invalid type across every In-family operator, even though the scalar
// itself is directly comparable through the same primitive an array
// element already uses.
func TestBareScalarValueComparesInsteadOfErroring(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	notin := NewNotInHandler(logr.Discard(), nil)
	anyin := NewAnyInHandler(logr.Discard(), nil)
	anynotin := NewAnyNotInHandler(logr.Discard(), nil)
	allin := NewAllInHandler(logr.Discard(), nil)
	allnotin := NewAllNotInHandler(logr.Discard(), nil)

	present := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{"scalar key, int value, present", "1", 1},
		{"array key, int value, present", []interface{}{"1"}, 1},
		{"scalar key, scalar key value, both int", 1, 1},
		{"scalar key, float value, present", "1.5", 1.5},
		{"scalar key, bool value, present", "true", true},
	}

	for _, tt := range present {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, in.Evaluate(tt.key, tt.value))
			assert.False(t, notin.Evaluate(tt.key, tt.value))
			assert.True(t, anyin.Evaluate(tt.key, tt.value))
			assert.False(t, anynotin.Evaluate(tt.key, tt.value))
			assert.True(t, allin.Evaluate(tt.key, tt.value))
			assert.False(t, allnotin.Evaluate(tt.key, tt.value))
		})
	}

	absent := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{"scalar key, int value, absent", "1", 2},
		{"array key, int value, absent", []interface{}{"1"}, 2},
		{"scalar key, scalar key value, absent", 1, 2},
	}

	for _, tt := range absent {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, in.Evaluate(tt.key, tt.value))
			assert.True(t, notin.Evaluate(tt.key, tt.value))
			assert.False(t, anyin.Evaluate(tt.key, tt.value))
			assert.True(t, anynotin.Evaluate(tt.key, tt.value))
			assert.False(t, allin.Evaluate(tt.key, tt.value))
			assert.True(t, allnotin.Evaluate(tt.key, tt.value))
		})
	}
}

// TestBareNonComparableValueFailsSafeWithoutPanic pins the #1039 correction:
// a bare value that is genuinely non-comparable (a map, or nil) must not
// panic, and its membership is undefined rather than false. Every In-family
// operator returns false, so in particular a NotIn/AnyNotIn/AllNotIn deny
// rule does NOT fire on absent or non-scalar data (the production casualty
// was an AnyNotIn over request.userInfo.groups that resolved to nil). This is
// three-valued (SQL NULL) semantics: In and NotIn are both false on an opaque
// value, so they are deliberately NOT complements here -- complementarity is
// scoped to comparable values in TestBareScalarValueComplementsHold.
func TestBareNonComparableValueFailsSafeWithoutPanic(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	notin := NewNotInHandler(logr.Discard(), nil)
	anyin := NewAnyInHandler(logr.Discard(), nil)
	anynotin := NewAnyNotInHandler(logr.Discard(), nil)
	allin := NewAllInHandler(logr.Discard(), nil)
	allnotin := NewAllNotInHandler(logr.Discard(), nil)

	cases := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{"scalar key vs bare map value", "1", map[string]interface{}{"a": "b"}},
		{"array key vs bare map value", []interface{}{"1"}, map[string]interface{}{"a": "b"}},
		{"scalar key vs nil value", "1", nil},
		{"array key vs nil value", []interface{}{"1"}, nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, in.Evaluate(tt.key, tt.value))
			assert.False(t, notin.Evaluate(tt.key, tt.value))
			assert.False(t, anyin.Evaluate(tt.key, tt.value))
			assert.False(t, anynotin.Evaluate(tt.key, tt.value))
			assert.False(t, allin.Evaluate(tt.key, tt.value))
			assert.False(t, allnotin.Evaluate(tt.key, tt.value))
		})
	}
}

// TestNotInFamilyDoesNotFireOnNullValue is the #1039 regression guard. A
// *NotIn deny rule whose "value" operand resolves to null -- for example an
// AnyNotIn over request.userInfo.groups when the request carries no userInfo,
// the confirmed production casualty -- must NOT fire. Before this fix it
// flipped to a deny and blocked workloads under Enforce. Comparable values are
// unaffected: a present, non-matching scalar still resolves to a deny.
func TestNotInFamilyDoesNotFireOnNullValue(t *testing.T) {
	notin := NewNotInHandler(logr.Discard(), nil)
	anynotin := NewAnyNotInHandler(logr.Discard(), nil)
	allnotin := NewAllNotInHandler(logr.Discard(), nil)

	key := []interface{}{"system:authenticated"}

	// null value: the deny must not fire
	assert.False(t, notin.Evaluate(key, nil))
	assert.False(t, anynotin.Evaluate(key, nil))
	assert.False(t, allnotin.Evaluate(key, nil))

	// a present, non-matching scalar value still resolves to a deny
	assert.True(t, notin.Evaluate(key, "other"))
	assert.True(t, anynotin.Evaluate(key, "other"))
	assert.True(t, allnotin.Evaluate(key, "other"))
}

// TestBareScalarValueComplementsHold re-asserts NotIn == !In (and the
// Any/All twins) specifically over the bare-scalar-value shape, across
// present and absent rows. Complementarity is scoped to comparable values on
// purpose: an opaque (null/map) value is three-valued and makes both sides
// false, which TestBareNonComparableValueFailsSafeWithoutPanic covers (#1039).
func TestBareScalarValueComplementsHold(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	notin := NewNotInHandler(logr.Discard(), nil)
	anyin := NewAnyInHandler(logr.Discard(), nil)
	anynotin := NewAnyNotInHandler(logr.Discard(), nil)
	allin := NewAllInHandler(logr.Discard(), nil)
	allnotin := NewAllNotInHandler(logr.Discard(), nil)

	cases := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{"present int", "1", 1},
		{"absent int", "1", 2},
		{"present float", "1.5", 1.5},
		{"present bool", "true", true},
		{"array key present", []interface{}{"1"}, 1},
		{"array key absent", []interface{}{"1"}, 2},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, in.Evaluate(tt.key, tt.value), !notin.Evaluate(tt.key, tt.value))
			assert.Equal(t, allin.Evaluate(tt.key, tt.value), !anynotin.Evaluate(tt.key, tt.value))
			assert.Equal(t, anyin.Evaluate(tt.key, tt.value), !allnotin.Evaluate(tt.key, tt.value))
		})
	}
}

// TestScalarValueMatchingStaysDirectional pins the base semantics for the
// scalar-value paths of the key-exists helpers: the "value" operand (a
// bare string, or a bare scalar) is the glob pattern, and the "key"
// operand is the concrete string being tested -- never the reverse. This
// is the exact shape base's keyExistsInArray/anyKeyExistsInArray/
// allKeyExistsInArray used (`wildcard.Match(value, key)`, one direction
// only). Symmetric matching here would let a resource-controlled key of
// "*" match any policy-declared scalar value.
func TestScalarValueMatchingStaysDirectional(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)
	anyin := NewAnyInHandler(logr.Discard(), nil)
	allin := NewAllInHandler(logr.Discard(), nil)

	// key is a wildcard, value is a concrete string: must NOT match.
	assert.False(t, in.Evaluate("*", "prod"))
	assert.False(t, anyin.Evaluate("*", "prod"))
	assert.False(t, allin.Evaluate("*", "prod"))

	// value is a wildcard, key is a concrete string: matches, as before.
	assert.True(t, in.Evaluate("prod", "prod*"))
	assert.True(t, anyin.Evaluate("prod", "prod*"))
	assert.True(t, allin.Evaluate("prod", "prod*"))

	// a literal key never matches a literal, differently-shaped value.
	assert.False(t, in.Evaluate("prod*", "prod"))
}

// TestArrayValueMatchingStaysSymmetric confirms the array-value ([]interface{})
// path did NOT change: base already matched it symmetrically
// (wildcard.Match(val, key) || wildcard.Match(key, val)), and that must be
// preserved even though the scalar-value path is now directional.
func TestArrayValueMatchingStaysSymmetric(t *testing.T) {
	in := NewInHandler(logr.Discard(), nil)

	// key "*" against an array value containing a concrete string: the
	// array-value path is symmetric at base, so this still matches.
	assert.True(t, in.Evaluate("*", []interface{}{"prod"}))
}
