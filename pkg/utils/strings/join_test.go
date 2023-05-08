package strings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoins(t *testing.T) {
	assert.Equal(t, "test", JoinNonEmpty([]string{"test", ""}, ","))
	assert.Equal(t, "test,test", JoinNonEmpty([]string{"test", "test"}, ","))
	assert.Equal(t, "test; test", JoinNonEmpty([]string{"test", "", "test", ""}, "; "))
	assert.Equal(t, "fi fo fum", JoinNonEmpty([]string{"", "fi", "", "fo", "", "fum"}, " "))
}
