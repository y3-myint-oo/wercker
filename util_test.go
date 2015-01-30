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
