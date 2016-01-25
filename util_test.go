package main

import (
	"encoding/hex"
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

func (s *UtilSuite) TestParseApplicationIDValid() {
	applicationID := "wercker/foobar"

	username, name, err := ParseApplicationID(applicationID)

	s.Equal(nil, err)
	s.Equal("wercker", username)
	s.Equal("foobar", name)
}

func (s *UtilSuite) TestParseApplicationIDInvalid() {
	applicationID := "foofoo"

	username, name, err := ParseApplicationID(applicationID)

	s.Error(err)
	s.Equal("", username)
	s.Equal("", name)
}

func (s *UtilSuite) TestParseApplicationIDInvalid2() {
	applicationID := "wercker/foobar/bla"

	username, name, err := ParseApplicationID(applicationID)

	s.Error(err)
	s.Equal("", username)
	s.Equal("", name)
}

func (s *UtilSuite) TestIsBuildIDValid() {
	buildID := "54e5dde34e104f675e007e3b"

	ok := IsBuildID(buildID)

	s.Equal(true, ok)
}

func (s *UtilSuite) TestIsBuildIDInvalid() {
	buildID := "54e5dde34e104f675e007e3"

	ok := IsBuildID(buildID)

	s.Equal(false, ok)
}

func (s *UtilSuite) TestIsBuildIDInvalid2() {
	buildID := "invalidinvalidinvalidinv"

	ok := IsBuildID(buildID)

	s.Equal(false, ok)
}

func (s *UtilSuite) TestIsBuildIDInvalid3() {
	buildID := "invalid"

	ok := IsBuildID(buildID)

	s.Equal(false, ok)
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

func (s *UtilSuite) TestGenerateDockerID() {
	id, err := GenerateDockerID()
	s.Require().NoError(err, "Unable to generate Docker ID")

	// The ID needs to be a valid hex value
	b, err := hex.DecodeString(id)
	s.Require().NoError(err, "Generated Docker ID was not a hex value")

	// The ID needs to be 256 bits
	s.Equal(256, len(b)*8)
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
	}

	for _, test := range testCases {
		actual := SplitSpaceOrComma(test.input)
		s.Equal(test.output, actual)
		s.Equal(len(test.output), 2)
	}
}
