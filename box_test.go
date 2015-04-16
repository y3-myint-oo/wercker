package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func boxByID(s string) (*Box, error) {
	return NewBox(&BoxConfig{ID: s}, emptyPipelineOptions(), &BoxOptions{})
}

func TestBoxName(t *testing.T) {
	_, err := boxByID("wercker/base@1.0.0")
	assert.NotNil(t, err)

	noTag, err := boxByID("wercker/base")
	assert.Nil(t, err)
	assert.Equal(t, "wercker/base:latest", noTag.Name)

	withTag, err := boxByID("wercker/base:foo")
	assert.Nil(t, err)
	assert.Equal(t, "wercker/base:foo", withTag.Name)
}
