package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironmentPassthru(t *testing.T) {
	env := NewEnvironment([]string{"X_PUBLIC=foo", "XXX_PRIVATE=bar", "NOT=included"})
	assert.Equal(t, 1, len(env.getPassthru()))
	assert.Equal(t, 1, len(env.getHiddenPassthru()))
}
