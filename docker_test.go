package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerNormalizeRegistry(t *testing.T) {
	quay := "https://quay.io/v1/"
	dock := "https://registry.hub.docker.com/v1/"
	assert.Equal(t, quay, normalizeRegistry("https://quay.io"))
	assert.Equal(t, quay, normalizeRegistry("https://quay.io/v1"))
	assert.Equal(t, quay, normalizeRegistry("http://quay.io/v1"))
	assert.Equal(t, quay, normalizeRegistry("https://quay.io/v1/"))
	assert.Equal(t, quay, normalizeRegistry("quay.io"))

	assert.Equal(t, dock, normalizeRegistry(""))
	assert.Equal(t, dock, normalizeRegistry("https://registry.hub.docker.com"))
	assert.Equal(t, dock, normalizeRegistry("http://registry.hub.docker.com"))
	assert.Equal(t, dock, normalizeRegistry("registry.hub.docker.com"))
}

func TestDockerNormalizeRepo(t *testing.T) {
	assert.Equal(t, "termie/gox-mirror", normalizeRepo("quay.io/termie/gox-mirror"))
	assert.Equal(t, "termie/gox-mirror", normalizeRepo("termie/gox-mirror"))
	assert.Equal(t, "mongo", normalizeRepo("mongo"))
}
