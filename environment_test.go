package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironmentPassthru(t *testing.T) {
	env := NewEnvironment("X_PUBLIC=foo", "XXX_PRIVATE=bar", "NOT=included")
	assert.Equal(t, 1, len(env.getPassthru()))
	assert.Equal(t, 1, len(env.getHiddenPassthru()))
}

func TestInterpolate(t *testing.T) {
	env := NewEnvironment("PUBLIC=foo")
	assert.Equal(t, env.Interpolate("$PUBLIC"), "foo")
	assert.Equal(t, env.Interpolate("one two $PUBLIC bar"), "one two foo bar")
}
