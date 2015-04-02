package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounterIncrement(t *testing.T) {
	counter := &Counter{}
	assert.Equal(t, 0, counter.Current, "expected counter to intialize with 0")

	n1 := counter.Increment()
	assert.Equal(t, 0, n1, "expected first increment to be 0")

	n2 := counter.Increment()
	assert.Equal(t, 1, n2, "expected second increment to be 0")
}

func TestCounterIncrement2(t *testing.T) {
	counter := &Counter{Current: 3}
	assert.Equal(t, 3, counter.Current, "expected counter to intialize with 3")

	n1 := counter.Increment()
	assert.Equal(t, 3, n1, "expected first increment to be 3")

	n2 := counter.Increment()
	assert.Equal(t, 4, n2, "expected second increment to be 4")
}

func TestParseApplicationIDValid(t *testing.T) {
	applicationID := "wercker/foobar"

	username, name, err := ParseApplicationID(applicationID)

	assert.Equal(t, nil, err)
	assert.Equal(t, "wercker", username)
	assert.Equal(t, "foobar", name)
}

func TestParseApplicationIDInvalid(t *testing.T) {
	applicationID := "foofoo"

	username, name, err := ParseApplicationID(applicationID)

	assert.Error(t, err)
	assert.Equal(t, "", username)
	assert.Equal(t, "", name)
}

func TestParseApplicationIDInvalid2(t *testing.T) {
	applicationID := "wercker/foobar/bla"

	username, name, err := ParseApplicationID(applicationID)

	assert.Error(t, err)
	assert.Equal(t, "", username)
	assert.Equal(t, "", name)
}

func TestIsBuildIDValid(t *testing.T) {
	buildID := "54e5dde34e104f675e007e3b"

	ok := IsBuildID(buildID)

	assert.Equal(t, true, ok)
}

func TestIsBuildIDInvalid(t *testing.T) {
	buildID := "54e5dde34e104f675e007e3"

	ok := IsBuildID(buildID)

	assert.Equal(t, false, ok)
}

func TestIsBuildIDInvalid2(t *testing.T) {
	buildID := "invalidinvalidinvalidinv"

	ok := IsBuildID(buildID)

	assert.Equal(t, false, ok)
}

func TestIsBuildIDInvalid3(t *testing.T) {
	buildID := "invalid"

	ok := IsBuildID(buildID)

	assert.Equal(t, false, ok)
}

func TestMinInt(t *testing.T) {
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

		assert.Equal(t, test.expected, actual)
	}
}

func TestMaxInt(t *testing.T) {
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

		assert.Equal(t, test.expected, actual)
	}
}
