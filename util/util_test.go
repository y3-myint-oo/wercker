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

package util

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type UtilSuite struct {
	*TestSuite
}

func TestUtilSuite(t *testing.T) {
	suiteTester := &UtilSuite{&TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *UtilSuite) TestCounterIncrement() {
	counter := &Counter{}
	s.Equal(0, counter.Current, "expected counter to intialize with 0")

	n1 := counter.Increment()
	s.Equal(0, n1, "expected first increment to be 0")

	n2 := counter.Increment()
	s.Equal(1, n2, "expected second increment to be 0")
}

func (s *UtilSuite) TestCounterIncrement2() {
	counter := &Counter{Current: 3}
	s.Equal(3, counter.Current, "expected counter to intialize with 3")

	n1 := counter.Increment()
	s.Equal(3, n1, "expected first increment to be 3")

	n2 := counter.Increment()
	s.Equal(4, n2, "expected second increment to be 4")
}

func (s *UtilSuite) TestMinInt() {
	testSteps := []struct {
		input    []int
		expected int
	}{
		{[]int{}, 0},
		{[]int{1}, 1},
		{[]int{1, 2}, 1},
		{[]int{2, 1}, 1},
		{[]int{1, 1}, 1},
		{[]int{5, 4, 3, 5, 7, 4}, 3},
	}

	for _, test := range testSteps {
		actual := MinInt(test.input...)

		s.Equal(test.expected, actual)
	}
}

func (s *UtilSuite) TestMaxInt() {
	testSteps := []struct {
		input    []int
		expected int
	}{
		{[]int{}, 0},
		{[]int{1}, 1},
		{[]int{1, 2}, 2},
		{[]int{2, 1}, 2},
		{[]int{1, 1}, 1},
		{[]int{5, 4, 3, 5, 7, 4}, 7},
	}

	for _, test := range testSteps {
		actual := MaxInt(test.input...)

		s.Equal(test.expected, actual)
	}
}

func (s *UtilSuite) TestSplitFunc() {
	testCases := []struct {
		input  string
		output []string
	}{
		{"hello, world", []string{"hello", "world"}},
		{"hello world", []string{"hello", "world"}},
		{"hello,              world", []string{"hello", "world"}},
		{"hello,world", []string{"hello", "world"}},
		{"hello                     world", []string{"hello", "world"}},
	}

	for _, test := range testCases {
		actual := SplitSpaceOrComma(test.input)
		s.Equal(test.output, actual)
		s.Equal(len(test.output), 2)
	}
}
