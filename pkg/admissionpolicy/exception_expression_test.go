package admissionpolicy

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       string
	}{
		{
			name:       "real selections",
			expression: "exceptions.allowedImages.size() + exceptions.allowedValues.size()",
			want:       "variables.allowedImages.size() + variables.allowedValues.size()",
		},
		{
			name:       "quoted strings stay literal",
			expression: `'exceptions.allowedImages' == 'exceptions.allowedValues'`,
			want:       `'exceptions.allowedImages' == 'exceptions.allowedValues'`,
		},
		{
			name:       "literal and reference in same expression",
			expression: `'exceptions.allowedImages' in exceptions.allowedImages`,
			want:       `"exceptions.allowedImages" in variables.allowedImages`,
		},
		{
			name:       "whitespace around field selector",
			expression: "exceptions . allowedImages.size()",
			want:       "variables.allowedImages.size()",
		},
		{
			name:       "similarly named resource fields stay unchanged",
			expression: "object.exceptions.allowedImages == object.exceptions.allowedImagesSuffix",
			want:       "object.exceptions.allowedImages == object.exceptions.allowedImagesSuffix",
		},
		{
			name:       "comprehension variable shadows exceptions",
			expression: "object.items.all(exceptions, exceptions.allowedImages.size() > 0)",
			want:       "object.items.all(exceptions, exceptions.allowedImages.size() > 0)",
		},
		{
			name:       "comprehension range uses global exceptions",
			expression: "exceptions.allowedImages.all(exceptions, exceptions != '')",
			want:       `variables.allowedImages.all(exceptions, exceptions != "")`,
		},
		{
			name:       "local binding shadows exceptions",
			expression: "cel.bind(exceptions, object, exceptions.allowedImages)",
			want:       "cel.bind(exceptions, object, exceptions.allowedImages)",
		},
		{
			name:       "binding initializer uses enclosing scope",
			expression: "cel.bind(exceptions, exceptions.allowedImages, exceptions)",
			want:       "cel.bind(exceptions, variables.allowedImages, exceptions)",
		},
		{
			name:       "presence test on always declared field",
			expression: "has(exceptions.allowedImages)",
			want:       "true",
		},
		{
			name:       "absolute name ignores local binding",
			expression: "object.items.all(exceptions, .exceptions.allowedImages.size() > 0)",
			want:       "object.items.all(exceptions, variables.allowedImages.size() > 0)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := replaceExpressions(tt.expression, map[string]string{
				"exceptions.allowedImages": "variables.allowedImages",
				"exceptions.allowedValues": "variables.allowedValues",
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplaceExpressions_DoesNotRewriteInsertedLiteralValues(t *testing.T) {
	expression, err := replaceExpressions(
		`exceptions.allowedImages[0] == 'exceptions.allowedValues' && exceptions.allowedValues[0] == 'blue'`,
		map[string]string{
			"exceptions.allowedImages": `["exceptions.allowedValues"]`,
			"exceptions.allowedValues": `["blue"]`,
		},
	)
	require.NoError(t, err)
	env, err := cel.NewEnv()
	require.NoError(t, err)
	ast, issues := env.Compile(expression)
	require.NoError(t, issues.Err())
	program, err := env.Program(ast)
	require.NoError(t, err)
	out, _, err := program.Eval(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, true, out.Value())
}

func TestReplaceExpressions_RejectsInvalidCEL(t *testing.T) {
	_, err := replaceExpressions("exceptions.allowedImages[", map[string]string{"exceptions.allowedImages": "[]"})
	require.ErrorContains(t, err, "failed to parse exception expression")
}

func TestReferencesGlobalExceptions(t *testing.T) {
	tests := []struct {
		name string
		expression string
		want bool
	}{
		{"bare identifier", "exceptions == null", true},
		{"supported global field", "exceptions.allowedImages.size() > 0", true},
		{"unknown global field", "exceptions.otherField", true},
		{"absolute identifier", ".exceptions.allowedValues", true},
		{"member call on global", "exceptions.size()", true},
		{"shadowed comprehension", "object.items.all(exceptions, exceptions != null)", false},
		{"shadowed field", "object.items.all(exceptions, exceptions.allowedImages.size() > 0)", false},
		{"global range with shadowed body", "exceptions.allowedImages.all(exceptions, exceptions != null)", true},
		{"local binding", "cel.bind(exceptions, object, exceptions.allowedImages)", false},
		{"initializer sees global", "cel.bind(exceptions, exceptions.allowedImages, exceptions)", true},
		{"string literal", "'exceptions.allowedImages'", false},
		{"resource field", "object.exceptions.allowedImages", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := referencesGlobalExceptions(tt.expression)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
	_, err := referencesGlobalExceptions("exceptions[")
	require.Error(t, err)
}
