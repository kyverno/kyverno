package deprecations

import (
	"bytes"
	"regexp"
	"testing"

	fatihcolor "github.com/fatih/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/stretchr/testify/assert"
)

var ansiRegex = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z0-9]*(?:;[a-zA-Z0-9]*)*)?\u0007)|(?:\\d{1,4}(?:;\\d{0,4})*)?[0-mG-Z])")

func TestDeprecationChecks_ColorEnabled(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})
	fatihcolor.NoColor = false
	color.Init(false)

	t.Run("CheckUserInfo", func(t *testing.T) {
		buf := &bytes.Buffer{}
		res := CheckUserInfo(buf, "test-user-info.yaml", &v1alpha1.UserInfo{})
		assert.True(t, res)
		assert.Contains(t, buf.String(), color.BoldYellow("WARNING:"))
		assert.True(t, ansiRegex.MatchString(buf.String()))
	})

	t.Run("CheckValues", func(t *testing.T) {
		buf := &bytes.Buffer{}
		res := CheckValues(buf, "test-values.yaml", &v1alpha1.Values{})
		assert.True(t, res)
		assert.Contains(t, buf.String(), color.BoldYellow("WARNING:"))
		assert.True(t, ansiRegex.MatchString(buf.String()))
	})

	t.Run("CheckTest", func(t *testing.T) {
		buf := &bytes.Buffer{}
		res := CheckTest(buf, "test-test.yaml", &v1alpha1.Test{})
		assert.True(t, res)
		assert.Contains(t, buf.String(), color.BoldYellow("WARNING:"))
		assert.True(t, ansiRegex.MatchString(buf.String()))
	})
}

func TestDeprecationChecks_ColorDisabled(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})
	fatihcolor.NoColor = true
	color.Init(true)

	t.Run("CheckUserInfo", func(t *testing.T) {
		buf := &bytes.Buffer{}
		res := CheckUserInfo(buf, "test-user-info.yaml", &v1alpha1.UserInfo{})
		assert.True(t, res)
		assert.Contains(t, buf.String(), "WARNING:")
		assert.False(t, ansiRegex.MatchString(buf.String()))
	})

	t.Run("CheckValues", func(t *testing.T) {
		buf := &bytes.Buffer{}
		res := CheckValues(buf, "test-values.yaml", &v1alpha1.Values{})
		assert.True(t, res)
		assert.Contains(t, buf.String(), "WARNING:")
		assert.False(t, ansiRegex.MatchString(buf.String()))
	})

	t.Run("CheckTest", func(t *testing.T) {
		buf := &bytes.Buffer{}
		res := CheckTest(buf, "test-test.yaml", &v1alpha1.Test{})
		assert.True(t, res)
		assert.Contains(t, buf.String(), "WARNING:")
		assert.False(t, ansiRegex.MatchString(buf.String()))
	})
}
