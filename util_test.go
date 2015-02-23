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

	username, name, ok := ParseApplicationID(applicationID)

	assert.Equal(t, true, ok)
	assert.Equal(t, "wercker", username)
	assert.Equal(t, "foobar", name)
}

func TestParseApplicationIDInvalid(t *testing.T) {
	applicationID := "foofoo"

	username, name, ok := ParseApplicationID(applicationID)

	assert.Equal(t, false, ok)
	assert.Equal(t, "", username)
	assert.Equal(t, "", name)
}

func TestParseApplicationIDInvalid2(t *testing.T) {
	applicationID := "wercker/foobar/bla"

	username, name, ok := ParseApplicationID(applicationID)

	assert.Equal(t, false, ok)
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
