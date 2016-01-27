//   Copyright 2016 Wercker Holding BV
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

// This is an interface and a helper to make it easier to construct our options
// objects for testing without literally parsing the flags we define in
// the sentcli/cmd package. Mostly it is a re-implementation of the codegangsta
// cli.Context interface that we actually use.

package util

import (
	"flag"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/stretchr/testify/suite"
)

func flagSet(name string, flags []cli.Flag) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)

	for _, f := range flags {
		f.Apply(set)
	}
	return set
}

func testContext() *cli.Context {
	set := flagSet("test", []cli.Flag{
		cli.IntFlag{Name: "globalexist"},
	})
	args := []string{"--globalexist=1"}
	set.Parse(args)
	return cli.NewContext(nil, set, set)
}

type CLISuite struct {
	TestSuite
}

func (s *CLISuite) SetupTest() {
	s.TestSuite.SetupTest()
}

func TestCLISuite(t *testing.T) {
	suiteTester := new(CLISuite)
	suite.Run(t, suiteTester)
}

func (s *CLISuite) TestCheapSettingsInt() {
	settings := &CheapSettings{map[string]interface{}{"exist": 1}}
	i, ok := settings.Int("exist")
	s.Equal(1, i)
	s.Equal(true, ok)
}

func (s *CLISuite) TestCheapSettingsIntNotExists() {
	settings := &CheapSettings{}
	i, ok := settings.Int("nonexist")
	s.Equal(0, i)
	s.Equal(false, ok)
}

func (s *CLISuite) TestCheapSettingsIntWrongType() {
	settings := &CheapSettings{map[string]interface{}{"wrongtype": "foo"}}
	i, ok := settings.Int("wrongtype")
	s.Equal(0, i)
	s.Equal(false, ok)
}

func (s *CLISuite) TestCLISettingsGlobalInt() {
	ctx := testContext()
	settings := &CLISettings{ctx, map[string]interface{}{}}
	i, ok := settings.GlobalInt("globalexist")
	s.Equal(1, i)
	s.Equal(true, ok)
}

func (s *CLISuite) TestCLISettingsGlobalIntOverride() {
	ctx := testContext()
	settings := &CLISettings{ctx, map[string]interface{}{"globalexist": 2}}
	i, ok := settings.GlobalInt("globalexist")
	s.Equal(2, i)
	s.Equal(true, ok)
}

func (s *CLISuite) TestCLISettingsIntNotExists() {
	ctx := testContext()
	settings := &CLISettings{ctx, map[string]interface{}{}}
	i, ok := settings.Int("nonexist")
	s.Equal(0, i)
	s.Equal(false, ok)
}

func (s *CLISuite) TestCLISettingsIntWrongType() {
	ctx := testContext()
	settings := &CLISettings{ctx, map[string]interface{}{"globalexist": "foo"}}
	i, ok := settings.Int("globalexist")
	s.Equal(0, i)
	s.Equal(false, ok)
}
