package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironmentPassthru(t *testing.T) {
	env := NewEnvironment("X_PUBLIC=foo", "XXX_PRIVATE=bar", "NOT=included")
	assert.Equal(t, 1, len(env.getPassthru().Ordered()))
	assert.Equal(t, 1, len(env.getHiddenPassthru().Ordered()))
}

func TestInterpolate(t *testing.T) {
	env := NewEnvironment("PUBLIC=foo", "X_PRIVATE=zed", "XXX_OTHER=otter")
	env.Update(env.getPassthru().Ordered())
	env.Hidden.Update(env.getHiddenPassthru().Ordered())
	tt := assert.New(t)
	// this is impossible to set because the order the variables are applied is
	// defined by the caller
	//env.Update([][]string{[]string{"X_PUBLIC", "bar"}})
	//tt.Equal(env.Interpolate("$PUBLIC"), "foo", "Non-prefixed should alias any X_ prefixed vars.")
	tt.Equal(env.Interpolate("${PUBLIC}"), "foo", "Alternate shell style vars should work.")

	// NB: stipping only works because we cann Update with the passthru
	// function above
	tt.Equal(env.Interpolate("$PRIVATE"), "zed", "Xs should be stripped.")
	tt.Equal(env.Interpolate("$OTHER"), "otter", "XXXs should be stripped.")
	tt.Equal(env.Interpolate("one two $PUBLIC bar"), "one two foo bar", "interpolation should work in middle of string.")
}

func TestOrdered(t *testing.T) {
	env := NewEnvironment("PUBLIC=foo", "X_PRIVATE=zed")
	expected := [][]string{[]string{"PUBLIC", "foo"}, []string{"X_PRIVATE", "zed"}}
	tt := assert.New(t)
	tt.Equal(env.Ordered(), expected)
}

func TestExport(t *testing.T) {
	env := NewEnvironment("PUBLIC=foo", "X_PRIVATE=zed")
	tt := assert.New(t)
	expected := []string{`export PUBLIC="foo"`, `export X_PRIVATE="zed"`}
	tt.Equal(env.Export(), expected)
}
