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
	"time"

	"github.com/codegangsta/cli"
)

// Settings mathces the cli.Context interface so we can make a cheap
// re-implementation for testing purposes.
type Settings interface {
	Int(string) (int, bool)
	Duration(string) (time.Duration, bool)
	Float64(string) (float64, bool)
	Bool(string) (bool, bool)
	BoolT(string) (bool, bool)
	String(string) (string, bool)
	StringSlice(string) ([]string, bool)
	IntSlice(string) ([]int, bool)

	GlobalInt(string) (int, bool)
	GlobalDuration(string) (time.Duration, bool)
	// NOTE(termie): for some reason not in cli.Context
	// GlobalFloat64(string) (float64, bool)
	GlobalBool(string) (bool, bool)
	// NOTE(termie): for some reason not in cli.Context
	// GlobalBoolT(string) (bool, bool)
	GlobalString(string) (string, bool)
	GlobalStringSlice(string) ([]string, bool)
	GlobalIntSlice(string) ([]int, bool)
}

// CheapSettings based on a map, returns val, ok
type CheapSettings struct {
	data map[string]interface{}
}

func (s *CheapSettings) Int(name string) (rv int, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(int); asserted {
			rv = v
			ok = true
		}
	}
	return rv, ok
}

func (s *CheapSettings) Duration(name string) (rv time.Duration, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(time.Duration); asserted {
			rv = v
			ok = true
		}
	}
	return rv, ok
}

func (s *CheapSettings) Float64(name string) (rv float64, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(float64); asserted {
			rv = v
			ok = true
		}
	}
	return rv, ok
}

func (s *CheapSettings) Bool(name string) (rv bool, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(bool); asserted {
			rv = v
			ok = true
		}
	}
	return rv, ok
}

// BoolT is true by default
func (s *CheapSettings) BoolT(name string) (rv bool, ok bool) {
	rv = true
	if d, found := s.data[name]; found {
		if v, asserted := d.(bool); asserted {
			rv = v
			ok = true
		}
	}
	return rv, ok
}

func (s *CheapSettings) String(name string) (rv string, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(string); asserted {
			rv = v
			ok = true
		}
	}
	return rv, ok
}

func (s *CheapSettings) StringSlice(name string) (rv []string, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.([]string); asserted {
			rv = v
			ok = true
		}
	}
	return rv, ok
}

func (s *CheapSettings) IntSlice(name string) (rv []int, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.([]int); asserted {
			rv = v
			ok = true
		}
	}
	return rv, ok
}

// All the Global versions to do the same thing as the non-global
func (s *CheapSettings) GlobalInt(name string) (rv int, ok bool) {
	return s.Int(name)
}

func (s *CheapSettings) GlobalDuration(name string) (rv time.Duration, ok bool) {
	return s.Duration(name)
}

func (s *CheapSettings) GlobalBool(name string) (rv bool, ok bool) {
	return s.Bool(name)
}

func (s *CheapSettings) GlobalString(name string) (rv string, ok bool) {
	return s.String(name)
}

func (s *CheapSettings) GlobalStringSlice(name string) (rv []string, ok bool) {
	return s.StringSlice(name)
}

func (s *CheapSettings) GlobalIntSlice(name string) (rv []int, ok bool) {
	return s.IntSlice(name)
}

// CLISettings is a wrapper on a cli.Context with a special "target" set
// in place of "Args().First()"
type CLISettings struct {
	c    *cli.Context
	data map[string]interface{}
}

func NewCLISettings(ctx *cli.Context) *CLISettings {
	return &CLISettings{ctx, map[string]interface{}{"target": ctx.Args().First()}}
}

func (s *CLISettings) Int(name string) (rv int, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(int); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.Int(name), s.c.IsSet(name)
}

func (s *CLISettings) Duration(name string) (rv time.Duration, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(time.Duration); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.Duration(name), s.c.IsSet(name)
}

func (s *CLISettings) Float64(name string) (rv float64, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(float64); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.Float64(name), s.c.IsSet(name)
}

func (s *CLISettings) Bool(name string) (rv bool, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(bool); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.Bool(name), s.c.IsSet(name)
}

func (s *CLISettings) BoolT(name string) (rv bool, ok bool) {
	rv = true
	if d, found := s.data[name]; found {
		if v, asserted := d.(bool); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.BoolT(name), s.c.IsSet(name)
}

func (s *CLISettings) String(name string) (rv string, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(string); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.String(name), s.c.IsSet(name)
}

func (s *CLISettings) StringSlice(name string) (rv []string, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.([]string); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.StringSlice(name), s.c.IsSet(name)
}

func (s *CLISettings) IntSlice(name string) (rv []int, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.([]int); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.IntSlice(name), s.c.IsSet(name)
}

func (s *CLISettings) GlobalInt(name string) (rv int, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(int); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.GlobalInt(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalDuration(name string) (rv time.Duration, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(time.Duration); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.GlobalDuration(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalBool(name string) (rv bool, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(bool); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.GlobalBool(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalString(name string) (rv string, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.(string); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.GlobalString(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalStringSlice(name string) (rv []string, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.([]string); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.GlobalStringSlice(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalIntSlice(name string) (rv []int, ok bool) {
	if d, found := s.data[name]; found {
		if v, asserted := d.([]int); asserted {
			rv = v
			ok = true
		}
		return rv, ok
	}
	return s.c.GlobalIntSlice(name), s.c.GlobalIsSet(name)
}
