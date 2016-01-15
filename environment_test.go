package main

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/sentcli/util"
)

type EnvironmentSuite struct {
	util.TestSuite
}

func (s *EnvironmentSuite) SetupTest() {
	s.TestSuite.SetupTest()
}

func TestEnvironmentSuite(t *testing.T) {
	suiteTester := new(EnvironmentSuite)
	suite.Run(t, suiteTester)
}

func (s *EnvironmentSuite) TestPassthru() {
	env := NewEnvironment("X_PUBLIC=foo", "XXX_PRIVATE=bar", "NOT=included")
	s.Equal(1, len(env.getPassthru().Ordered()))
	s.Equal(1, len(env.getHiddenPassthru().Ordered()))
}

func (s *EnvironmentSuite) TestInterpolate() {
	env := NewEnvironment("PUBLIC=foo", "X_PRIVATE=zed", "XXX_OTHER=otter")
	env.Update(env.getPassthru().Ordered())
	env.Hidden.Update(env.getHiddenPassthru().Ordered())

	// this is impossible to set because the order the variables are applied is
	// defined by the caller
	//env.Update([][]string{[]string{"X_PUBLIC", "bar"}})
	//tt.Equal(env.Interpolate("$PUBLIC"), "foo", "Non-prefixed should alias any X_ prefixed vars.")
	s.Equal(env.Interpolate("${PUBLIC}"), "foo", "Alternate shell style vars should work.")

	// NB: stipping only works because we cann Update with the passthru
	// function above
	s.Equal(env.Interpolate("$PRIVATE"), "zed", "Xs should be stripped.")
	s.Equal(env.Interpolate("$OTHER"), "otter", "XXXs should be stripped.")
	s.Equal(env.Interpolate("one two $PUBLIC bar"), "one two foo bar", "interpolation should work in middle of string.")
}

func (s *EnvironmentSuite) TestOrdered() {
	env := NewEnvironment("PUBLIC=foo", "X_PRIVATE=zed")
	expected := [][]string{[]string{"PUBLIC", "foo"}, []string{"X_PRIVATE", "zed"}}
	s.Equal(env.Ordered(), expected)
}

func (s *EnvironmentSuite) TestExport() {
	env := NewEnvironment("PUBLIC=foo", "X_PRIVATE=zed")
	expected := []string{`export PUBLIC="foo"`, `export X_PRIVATE="zed"`}
	s.Equal(env.Export(), expected)
}
